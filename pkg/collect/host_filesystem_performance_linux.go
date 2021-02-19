package collect

import (
	"encoding/json"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
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

func HostFilesystemPerformance(c *HostCollector) (map[string][]byte, error) {
	var operationSize uint64 = 1024
	if c.Collect.FilesystemPerformance.OperationSizeBytes != 0 {
		operationSize = c.Collect.FilesystemPerformance.OperationSizeBytes
	}

	var fileSize uint64 = 10 * 1024 * 1024
	if c.Collect.FilesystemPerformance.FileSize != "" {
		quantity, err := resource.ParseQuantity(c.Collect.FilesystemPerformance.FileSize)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to parse fileSize %q", c.Collect.FilesystemPerformance.FileSize)
		}
		fileSizeInt64, ok := quantity.AsInt64()
		if !ok {
			return nil, errors.Wrapf(err, "failed to parse fileSize %q", c.Collect.FilesystemPerformance.FileSize)
		}
		fileSize = uint64(fileSizeInt64)
	}

	if c.Collect.FilesystemPerformance.Directory == "" {
		return nil, errors.New("Directory is required to collect filesystem performance info")
	}
	if err := os.MkdirAll(c.Collect.FilesystemPerformance.Directory, 0700); err != nil {
		return nil, errors.Wrapf(err, "failed to mkdir %q", c.Collect.FilesystemPerformance.Directory)
	}
	filename := filepath.Join(c.Collect.FilesystemPerformance.Directory, "fsperf")

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
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
		if c.Collect.FilesystemPerformance.Sync {
			if err := f.Sync(); err != nil {
				return nil, errors.Wrapf(err, "sync %s", filename)
			}
		} else if c.Collect.FilesystemPerformance.Datasync {
			if err := syscall.Fdatasync(int(f.Fd())); err != nil {
				return nil, errors.Wrapf(err, "datasync %s", filename)
			}
		}

		d := time.Now().Sub(start)
		results = append(results, d)

		written += uint64(n)
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

	// Random IOPS benchmark

	// Re-open the file read+write in direct mode to prevent caching
	if err := f.Close(); err != nil {
		return nil, errors.Wrapf(err, "close %s", filename)
	}
	f, err = os.OpenFile(filename, os.O_RDWR|syscall.O_DIRECT, 0600)
	if err != nil {
		return nil, errors.Wrapf(err, "open direct %s", filename)
	}

	offsets := make([]int64, len(results))

	for index, p := range rand.Perm(len(results)) {
		offsets[index] = int64(p) * int64(operationSize)
	}

	// Use multiple workers to keep the filesystem busy. Since operations are serialized on a single
	// file, more than 2 does not improve IOPS.
	workers := 2
	wg := sync.WaitGroup{}
	m := sync.Mutex{}

	errs := make(chan error, workers)

	start := time.Now()

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			data := make([]byte, int(operationSize))
			fd := int(f.Fd())

			for idx, offset := range offsets {
				if idx%workers != i {
					continue
				}

				m.Lock()
				n, err := syscall.Pread(fd, data, offset)
				m.Unlock()

				if err != nil {
					errs <- errors.Wrapf(err, "failed to pread %d bytes to %s at offset %d", len(data), filename, offset)
				}
				if n != len(data) {
					errs <- errors.Wrapf(err, "pread %d of %d bytes to %s at offset %d", n, len(data), filename, offset)
				}
			}
		}(i)
	}

	wg.Wait()
	if len(errs) > 0 {
		return nil, <-errs
	}

	d := time.Now().Sub(start)
	nsPerIO := d / time.Duration(len(offsets))
	iops := time.Second / nsPerIO

	fsPerf.IOPS = int(iops)

	collectorName := c.Collect.FilesystemPerformance.CollectorName
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
