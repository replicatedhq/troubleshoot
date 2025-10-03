package collect

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/klog/v2"
)

// Today we only care about checking for write latency so the options struct
// only has what we need for that.  we'll collect all the results from a single run of fio
// and filter out the fsync results for analysis.  TODO: update the analyzer so any/all results
// from fio can be analyzed.

func collectHostFilesystemPerformance(hostCollector *troubleshootv1beta2.FilesystemPerformance, bundlePath string) (map[string][]byte, error) {
	timeout := time.Minute

	if hostCollector.Timeout != "" {
		d, err := time.ParseDuration(hostCollector.Timeout)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse timeout %q", hostCollector.Timeout)
		}
		timeout = d
	}
	timeoutCtx, timeoutCancel := context.WithTimeout(context.Background(), timeout)
	defer timeoutCancel()

	// Start a new context for the fio collector and handle the timeout separately so we can
	// distinguish between the timeout and a command failure in the analyzer.
	collectCtx, collectCancel := context.WithCancel(context.Background())
	defer collectCancel()

	collectorName := hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "filesystemPerformance"
	}
	name := filepath.Join("host-collectors/filesystemPerformance", collectorName+".json")

	if err := os.MkdirAll(hostCollector.Directory, 0700); err != nil {
		return nil, errors.Wrapf(err, "failed to mkdir %q", hostCollector.Directory)
	}

	// Start the background IOPS task and wait for warmup
	if hostCollector.EnableBackgroundIOPS {
		// The done channel waits for all jobs to delete their work file after the context is
		// canceled
		jobs := hostCollector.BackgroundReadIOPSJobs + hostCollector.BackgroundWriteIOPSJobs
		done := make(chan bool, jobs)
		defer func() {
			collectCancel()
			for i := 0; i < jobs; i++ {
				<-done
			}
		}()

		opts := backgroundIOPSOpts{
			read:      true,
			iopsLimit: hostCollector.BackgroundReadIOPS,
			jobs:      hostCollector.BackgroundReadIOPSJobs,
			directory: hostCollector.Directory,
		}
		backgroundIOPS(collectCtx, opts, done)

		opts = backgroundIOPSOpts{
			read:      false,
			iopsLimit: hostCollector.BackgroundWriteIOPS,
			jobs:      hostCollector.BackgroundWriteIOPSJobs,
			directory: hostCollector.Directory,
		}
		backgroundIOPS(collectCtx, opts, done)

		time.Sleep(time.Second * time.Duration(hostCollector.BackgroundIOPSWarmupSeconds))
	}

	var fioResult *FioResult
	var fioStderr []byte
	errCh := make(chan error, 1)
	go func() {
		var err error
		fioResult, fioStderr, err = collectFioResults(collectCtx, hostCollector)
		errCh <- err
	}()

	select {
	case <-timeoutCtx.Done():
		return nil, errors.New("timeout")
	case err := <-errCh:
		if err != nil {
			return nil, errors.Wrap(err, "failed to collect fio results")
		}
	}

	b, err := json.Marshal(fioResult)
	if err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal fio results")
	}

	output := NewResult()
	output.SaveResult(bundlePath, name, bytes.NewBuffer(b))

	// Save stderr if present (captures permission errors and warnings)
	if len(fioStderr) > 0 {
		stderrPath := strings.TrimSuffix(name, filepath.Ext(name)) + "-stderr.txt"
		klog.V(2).Infof("Saving filesystem performance stderr to %q in bundle", stderrPath)
		output.SaveResult(bundlePath, stderrPath, bytes.NewBuffer(fioStderr))
	}

	return output, nil
}

type backgroundIOPSOpts struct {
	jobs      int
	iopsLimit int
	read      bool
	directory string
}

func backgroundIOPS(ctx context.Context, opts backgroundIOPSOpts, done chan bool) {
	r := rand.New(rand.NewSource(time.Now().Unix()))

	// Waits until files are prepared before returning
	var wg sync.WaitGroup

	// Rate limit IOPS with fixed window. Every second is a new window. All jobs increment
	// the same counter before performing an operation. If the counter for the current
	// window has already reached the IOPS limit then sleep until the beginning of the next
	// window.
	windows := map[int64]int{}
	var mtx sync.Mutex

	for i := 0; i < opts.jobs; i++ {
		wg.Add(1)

		go func(i int) {
			filename := fmt.Sprintf("background-write-%d", i)
			if opts.read {
				filename = fmt.Sprintf("background-read-%d", i)
			}
			filename = filepath.Join(opts.directory, filename)
			// Ensure we signal completion exactly once per job
			defer func() { done <- true }()
			f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|syscall.O_DIRECT, 0600)
			if err != nil {
				log.Printf("Failed to create temp file for background IOPS job: %v", err)
				wg.Done() // Signal that this job's initialization is complete (even though it failed)
				return
			}
			defer func() {
				if err := os.Remove(filename); err != nil {
					log.Println(err.Error())
				}
			}()

			// For O_DIRECT I/O must be aligned on the sector size of the underlying block device.
			// Usually that's 512 but may also be 4096. Use 4096 since that works in either case.
			opSize := 4096
			blocks := 25600
			fileSize := int64(opSize * blocks)

			if opts.read {
				_, err := io.Copy(f, io.LimitReader(r, fileSize))
				if err != nil {
					log.Printf("Failed to write temp file for background read IOPS jobs: %v", err)
					wg.Done() // Signal that this job's initialization is complete (even though it failed)
					return
				}
			} else {
				err := f.Truncate(int64(opSize * blocks))
				if err != nil {
					log.Printf("Failed to resize temp file for backgroupd write IOPS jobs: %v", err)
				}
			}

			wg.Done()
			for {
				if ctx.Err() != nil {
					return
				}

				mtx.Lock()
				windowKey := time.Now().Unix()

				if windows[windowKey] >= opts.iopsLimit {
					mtx.Unlock()

					nextWindow := windowKey + 1
					timeUntilNextWindow := time.Until(time.Unix(nextWindow, 0))
					time.Sleep(timeUntilNextWindow)
					continue
				}

				windows[windowKey]++
				mtx.Unlock()

				blockOffset := rand.Intn(blocks)
				offset := int64(blockOffset) * int64(opSize)
				data := make([]byte, opSize)

				if opts.read {
					syscall.Pread(int(f.Fd()), data, offset)
				} else {
					if _, err := rand.Read(data); err != nil {
						log.Printf("Failed to generate data for background IOPS write job: %v", err)
						return
					}
					syscall.Pwrite(int(f.Fd()), data, offset)
				}
			}
		}(i)
	}

	wg.Wait()
}
