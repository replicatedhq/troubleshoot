package rrd

import (
	"errors"
	"strconv"
	"time"
)

type cstring byte

func newCstring(s string) *cstring {
	return nil
}

func (cs *cstring) Free() {
	return
}

func (cs *cstring) String() string {
	return ""
}

func (c *Creator) create() error {
	return errors.New("not implemented")
}

func (u *Updater) update(args []*cstring) error {
	return errors.New("not implemented")
}

func ftoa(f float64) string {
	return strconv.FormatFloat(f, 'e', 10, 64)
}

func i64toa(i int64) string {
	return strconv.FormatInt(i, 10)
}

func u64toa(u uint64) string {
	return ""
}

func itoa(i int) string {
	return ""
}

func utoa(u uint) string {
	return ""
}

func parseInfoKey(ik string) (kname, kkey string, kid int) {
	return
}

func (g *Grapher) graph(filename string, start, end time.Time) (GraphInfo, []byte, error) {
	return GraphInfo{}, nil, errors.New("not implemented")
}

// Info returns information about RRD file.
func Info(filename string) (map[string]interface{}, error) {
	return nil, errors.New("not implemented")
}

// Fetch retrieves data from RRD file.
func Fetch(filename, cf string, start, end time.Time, step time.Duration) (FetchResult, error) {
	return FetchResult{}, errors.New("not implemented")
}

// FreeValues free values memory allocated by C.
func (r *FetchResult) FreeValues() {
}

// Values returns copy of internal array of values.
func (r *FetchResult) Values() []float64 {
	return nil
}

// Export data from RRD file(s)
func (e *Exporter) xport(start, end time.Time, step time.Duration) (XportResult, error) {
	return XportResult{}, errors.New("not implemented")
}

// FreeValues free values memory allocated by C.
func (r *XportResult) FreeValues() {
}
