package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"k8s.io/apimachinery/pkg/api/resource"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

type Durations []time.Duration

func (d Durations) Len() int {
	return len(d)
}

func (d Durations) Less(i, j int) bool {
	return d[i] < d[j]
}

func (d Durations) Swap(i, j int) {
	d[i], d[j] = d[j], d[i]
}

func collectHostFilesystemPerformance(hostCollector *troubleshootv1beta2.FilesystemPerformance) (map[string][]byte, error) {
	timeout := time.Minute
	if hostCollector.Timeout != "" {
		d, err := time.ParseDuration(hostCollector.Timeout)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse timeout %q", hostCollector.Timeout)
		}
		timeout = d
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var operationSize uint64 = 1024
	if hostCollector.OperationSizeBytes != 0 {
		operationSize = hostCollector.OperationSizeBytes
	}

	var fileSize uint64 = 10 * 1024 * 1024
	if hostCollector.FileSize != "" {
		quantity, err := resource.ParseQuantity(hostCollector.FileSize)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse fileSize %q", hostCollector.FileSize)
		}
		fileSizeInt64, ok := quantity.AsInt64()
		if !ok {
			return nil, errors.Wrapf(err, "failed to parse fileSize %q", hostCollector.FileSize)
		}
		fileSize = uint64(fileSizeInt64)
	}

	if hostCollector.Directory == "" {
		return nil, errors.New("Directory is required to collect filesystem performance info")
	}
	// TODO: clean up this directory if its created
	if err := os.MkdirAll(hostCollector.Directory, 0700); err != nil {
		return nil, errors.Wrapf(err, "failed to mkdir %q", hostCollector.Directory)
	}
	filename := filepath.Join(hostCollector.Directory, "fsperf")

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		log.Panic(err)
		return nil, errors.Wrapf(err, "open %s", filename)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Println(err.Error())
		}
		if err := os.Remove(filename); err != nil {
			log.Println(err.Error())
		}
	}()

	// Start the background IOPS task and wait for warmup
	if hostCollector.EnableBackgroundIOPS {
		// The done channel waits for all jobs to delete their work file after the context is
		// canceled
		jobs := hostCollector.BackgroundReadIOPSJobs + hostCollector.BackgroundWriteIOPSJobs
		done := make(chan bool, jobs)
		defer func() {
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
		backgroundIOPS(ctx, opts, done)

		opts = backgroundIOPSOpts{
			read:      false,
			iopsLimit: hostCollector.BackgroundWriteIOPS,
			jobs:      hostCollector.BackgroundWriteIOPSJobs,
			directory: hostCollector.Directory,
		}
		backgroundIOPS(ctx, opts, done)

		time.Sleep(time.Second * time.Duration(hostCollector.BackgroundIOPSWarmupSeconds))
	}

	// Sequential writes benchmark
	var written uint64 = 0
	var results Durations

	for {
		if written >= fileSize {
			break
		}

		data := make([]byte, int(operationSize))
		rand.Read(data)

		start := time.Now()

		n, err := f.Write(data)
		if err != nil {
			return nil, errors.Wrapf(err, "write to %s", filename)
		}
		if hostCollector.Sync {
			if err := f.Sync(); err != nil {
				return nil, errors.Wrapf(err, "sync %s", filename)
			}
		} else if hostCollector.Datasync {
			if err := syscall.Fdatasync(int(f.Fd())); err != nil {
				return nil, errors.Wrapf(err, "datasync %s", filename)
			}
		}

		d := time.Now().Sub(start)
		results = append(results, d)

		written += uint64(n)

		if ctx.Err() != nil {
			break
		}
	}

	if len(results) == 0 {
		return nil, errors.New("No filesystem performance results collected")
	}

	sort.Sort(results)

	var sum time.Duration
	for _, d := range results {
		sum += d
	}

	fsPerf := &FSPerfResults{
		Min:     results[0],
		Max:     results[len(results)-1],
		Average: sum / time.Duration(len(results)),
		P1:      results[getPercentileIndex(.01, len(results))],
		P5:      results[getPercentileIndex(.05, len(results))],
		P10:     results[getPercentileIndex(.1, len(results))],
		P20:     results[getPercentileIndex(.2, len(results))],
		P30:     results[getPercentileIndex(.3, len(results))],
		P40:     results[getPercentileIndex(.4, len(results))],
		P50:     results[getPercentileIndex(.5, len(results))],
		P60:     results[getPercentileIndex(.6, len(results))],
		P70:     results[getPercentileIndex(.7, len(results))],
		P80:     results[getPercentileIndex(.8, len(results))],
		P90:     results[getPercentileIndex(.9, len(results))],
		P95:     results[getPercentileIndex(.95, len(results))],
		P99:     results[getPercentileIndex(.99, len(results))],
		P995:    results[getPercentileIndex(.995, len(results))],
		P999:    results[getPercentileIndex(.999, len(results))],
		P9995:   results[getPercentileIndex(.9995, len(results))],
		P9999:   results[getPercentileIndex(.9999, len(results))],
	}

	collectorName := hostCollector.CollectorName
	if collectorName == "" {
		collectorName = "filesystemPerformance"
	}
	name := filepath.Join("filesystemPerformance", collectorName+".json")
	b, err := json.Marshal(fsPerf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal fs perf results")
	}

	return map[string][]byte{
		name: b,
	}, nil
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
			f, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC|syscall.O_DIRECT, 0600)
			if err != nil {
				log.Printf("Failed to create temp file for background IOPS job: %v", err)
				done <- true
				return
			}
			defer func() {
				if err := os.Remove(filename); err != nil {
					log.Println(err.Error())
				}
				done <- true
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
