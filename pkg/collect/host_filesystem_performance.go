package collect

import (
	"crypto/rand"
	"encoding/json"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/resource"
)

type FSPerfResults struct {
	Min   time.Duration
	Max   time.Duration
	P1    time.Duration
	P5    time.Duration
	P10   time.Duration
	P20   time.Duration
	P30   time.Duration
	P40   time.Duration
	P50   time.Duration
	P60   time.Duration
	P70   time.Duration
	P80   time.Duration
	P90   time.Duration
	P95   time.Duration
	P99   time.Duration
	P995  time.Duration
	P999  time.Duration
	P9995 time.Duration
	P9999 time.Duration
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

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
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

	fsPerf := &FSPerfResults{
		Min:   results[0],
		Max:   results[len(results)-1],
		P1:    results[getPercentileIndex(.01, len(results))],
		P5:    results[getPercentileIndex(.05, len(results))],
		P10:   results[getPercentileIndex(.1, len(results))],
		P20:   results[getPercentileIndex(.2, len(results))],
		P30:   results[getPercentileIndex(.3, len(results))],
		P40:   results[getPercentileIndex(.4, len(results))],
		P50:   results[getPercentileIndex(.5, len(results))],
		P60:   results[getPercentileIndex(.6, len(results))],
		P70:   results[getPercentileIndex(.7, len(results))],
		P80:   results[getPercentileIndex(.8, len(results))],
		P90:   results[getPercentileIndex(.9, len(results))],
		P95:   results[getPercentileIndex(.95, len(results))],
		P99:   results[getPercentileIndex(.99, len(results))],
		P995:  results[getPercentileIndex(.995, len(results))],
		P999:  results[getPercentileIndex(.999, len(results))],
		P9995: results[getPercentileIndex(.9995, len(results))],
		P9999: results[getPercentileIndex(.9999, len(results))],
	}

	collectorName := c.Collect.FilesystemPerformance.CollectorName
	if collectorName == "" {
		collectorName = "filesystemPerformance"
	}
	name := filepath.Join("filesystemPerformance", collectorName+".json")
	b, err := json.Marshal(fsPerf)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marsh fs perf results")
	}

	return map[string][]byte{
		name: b,
	}, nil
}

func getPercentileIndex(p float64, items int) int {
	if p >= 1 {
		return items - 1
	}
	return int(math.Ceil(p*float64(items))) - 1
}
