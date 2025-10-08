package collect

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"golang.org/x/net/context"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/klog/v2"
)

const (
	FioJobName        = "fsperf"
	DefaultFioRunTime = "120"
)

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

type CollectHostFilesystemPerformance struct {
	hostCollector *troubleshootv1beta2.FilesystemPerformance
	BundlePath    string
}

func (c *CollectHostFilesystemPerformance) Title() string {
	return hostCollectorTitleOrDefault(c.hostCollector.HostCollectorMeta, "Filesystem Performance")
}

func (c *CollectHostFilesystemPerformance) IsExcluded() (bool, error) {
	return isExcluded(c.hostCollector.Exclude)
}

func (c *CollectHostFilesystemPerformance) Collect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return collectHostFilesystemPerformance(c.hostCollector, c.BundlePath)
}

type FSPerfResults struct {
	Min     time.Duration
	Max     time.Duration
	Average time.Duration
	P1      time.Duration
	P5      time.Duration
	P10     time.Duration
	P20     time.Duration
	P30     time.Duration
	P40     time.Duration
	P50     time.Duration
	P60     time.Duration
	P70     time.Duration
	P80     time.Duration
	P90     time.Duration
	P95     time.Duration
	P99     time.Duration
	P995    time.Duration
	P999    time.Duration
	P9995   time.Duration
	P9999   time.Duration
}

func getPercentileIndex(p float64, items int) int {
	if p >= 1 {
		return items - 1
	}
	return int(math.Ceil(p*float64(items))) - 1
}

var fsPerfTmpl = template.Must(template.New("").Parse(`
   Min: {{ .Min }}
   Max: {{ .Max }}
   Avg: {{ .Average }}
    p1: {{ .P1 }}
    p5: {{ .P5 }}
   p10: {{ .P10 }}
   p20: {{ .P20 }}
   p30: {{ .P30 }}
   p40: {{ .P40 }}
   p50: {{ .P50 }}
   p60: {{ .P60 }}
   p70: {{ .P70 }}
   p80: {{ .P80 }}
   p90: {{ .P90 }}
   p95: {{ .P95 }}
   p99: {{ .P99 }}
 p99.5: {{ .P995 }}
 p99.9: {{ .P999 }}
p99.95: {{ .P9995 }}
p99.99: {{ .P9999 }}`))

func (f FSPerfResults) String() string {
	var buf bytes.Buffer

	fsPerfTmpl.Execute(&buf, f)

	return buf.String()
}

type FioResult struct {
	FioVersion    string           `json:"fio version,omitempty"`
	Timestamp     int64            `json:"timestamp,omitempty"`
	TimestampMS   int64            `json:"timestamp_ms,omitempty"`
	Time          string           `json:"time,omitempty"`
	GlobalOptions FioGlobalOptions `json:"global options,omitempty"`
	Jobs          []FioJobs        `json:"jobs,omitempty"`
	DiskUtil      []FioDiskUtil    `json:"disk_util,omitempty"`
}

func (f FioResult) String() string {
	var res string
	res += fmt.Sprintf("FIO version - %s\n", f.FioVersion)
	res += fmt.Sprintf("Global options - %s\n\n", f.GlobalOptions)
	for _, job := range f.Jobs {
		res += fmt.Sprintf("%s\n", job)
	}
	res += "Disk stats (read/write):\n"
	for _, du := range f.DiskUtil {
		res += fmt.Sprintf("%s\n", du)
	}

	return res
}

type FioGlobalOptions struct {
	Directory  string `json:"directory,omitempty"`
	RandRepeat string `json:"randrepeat,omitempty"`
	Verify     string `json:"verify,omitempty"`
	IOEngine   string `json:"ioengine,omitempty"`
	Direct     string `json:"direct,omitempty"`
	GtodReduce string `json:"gtod_reduce,omitempty"`
}

func (g FioGlobalOptions) String() string {
	return fmt.Sprintf("ioengine=%s verify=%s direct=%s gtod_reduce=%s", g.IOEngine, g.Verify, g.Direct, g.GtodReduce)
}

type FioJobs struct {
	JobName           string        `json:"jobname,omitempty"`
	GroupID           int           `json:"groupid,omitempty"`
	Error             int           `json:"error,omitempty"`
	Eta               int           `json:"eta,omitempty"`
	Elapsed           int           `json:"elapsed,omitempty"`
	JobOptions        FioJobOptions `json:"job options,omitempty"`
	Read              FioStats      `json:"read,omitempty"`
	Write             FioStats      `json:"write,omitempty"`
	Trim              FioStats      `json:"trim,omitempty"`
	Sync              FioStats      `json:"sync,omitempty"`
	JobRuntime        int32         `json:"job_runtime,omitempty"`
	UsrCpu            float32       `json:"usr_cpu,omitempty"`
	SysCpu            float32       `json:"sys_cpu,omitempty"`
	Ctx               int32         `json:"ctx,omitempty"`
	MajF              int32         `json:"majf,omitempty"`
	MinF              int32         `json:"minf,omitempty"`
	IoDepthLevel      FioDepth      `json:"iodepth_level,omitempty"`
	IoDepthSubmit     FioDepth      `json:"iodepth_submit,omitempty"`
	IoDepthComplete   FioDepth      `json:"iodepth_complete,omitempty"`
	LatencyNs         FioLatency    `json:"latency_ns,omitempty"`
	LatencyUs         FioLatency    `json:"latency_us,omitempty"`
	LatencyMs         FioLatency    `json:"latency_ms,omitempty"`
	LatencyDepth      int32         `json:"latency_depth,omitempty"`
	LatencyTarget     int32         `json:"latency_target,omitempty"`
	LatencyPercentile float32       `json:"latency_percentile,omitempty"`
	LatencyWindow     int32         `json:"latency_window,omitempty"`
}

func (j FioJobs) String() string {
	var job string
	job += fmt.Sprintf("%s\n", j.JobOptions)
	if j.Read.Iops != 0 || j.Read.BW != 0 {
		job += fmt.Sprintf("read:\n%s\n", j.Read)
	}
	if j.Write.Iops != 0 || j.Write.BW != 0 {
		job += fmt.Sprintf("write:\n%s\n", j.Write)
	}
	return job
}

type FioJobOptions struct {
	Name      string `json:"name,omitempty"`
	BS        string `json:"bs,omitempty"`
	Directory string `json:"directory,omitempty"`
	RW        string `json:"rw,omitempty"`
	IOEngine  string `json:"ioengine,omitempty"`
	FDataSync string `json:"fdatasync,omitempty"`
	Size      string `json:"size,omitempty"`
	RunTime   string `json:"runtime,omitempty"`
}

func (o FioJobOptions) String() string {
	return fmt.Sprintf("JobName: %s\n  blocksize=%s filesize=%s rw=%s", o.Name, o.BS, o.Size, o.RW)
}

type FioStats struct {
	IOBytes     int64         `json:"io_bytes,omitempty"`
	IOKBytes    int64         `json:"io_kbytes,omitempty"`
	BWBytes     int64         `json:"bw_bytes,omitempty"`
	BW          int64         `json:"bw,omitempty"`
	Iops        float32       `json:"iops,omitempty"`
	Runtime     int64         `json:"runtime,omitempty"`
	TotalIos    int64         `json:"total_ios,omitempty"`
	ShortIos    int64         `json:"short_ios,omitempty"`
	DropIos     int64         `json:"drop_ios,omitempty"`
	SlatNs      FioNS         `json:"slat_ns,omitempty"`
	ClatNs      FioNS         `json:"clat_ns,omitempty"`
	LatNs       FioNS         `json:"lat_ns,omitempty"`
	Percentile  FioPercentile `json:"percentile,omitempty"`
	BwMin       int64         `json:"bw_min,omitempty"`
	BwMax       int64         `json:"bw_max,omitempty"`
	BwAgg       float32       `json:"bw_agg,omitempty"`
	BwMean      float32       `json:"bw_mean,omitempty"`
	BwDev       float32       `json:"bw_dev,omitempty"`
	BwSamples   int32         `json:"bw_samples,omitempty"`
	IopsMin     int32         `json:"iops_min,omitempty"`
	IopsMax     int32         `json:"iops_max,omitempty"`
	IopsMean    float32       `json:"iops_mean,omitempty"`
	IopsStdDev  float32       `json:"iops_stddev,omitempty"`
	IopsSamples int32         `json:"iops_samples,omitempty"`
}

func (s FioStats) String() string {
	var stats string
	stats += fmt.Sprintf("  IOPS=%f BW(KiB/s)=%d\n", s.Iops, s.BW)
	stats += fmt.Sprintf("  iops: min=%d max=%d avg=%f\n", s.IopsMin, s.IopsMax, s.IopsMean)
	stats += fmt.Sprintf("  bw(KiB/s): min=%d max=%d avg=%f", s.BwMin, s.BwMax, s.BwMean)
	return stats
}

func (s FioStats) FSPerfResults() FSPerfResults {
	return FSPerfResults{
		Min:     time.Duration(s.LatNs.Min),
		Max:     time.Duration(s.LatNs.Max),
		Average: time.Duration(s.LatNs.Mean),
		P1:      time.Duration(s.LatNs.Percentile.P1),
		P5:      time.Duration(s.LatNs.Percentile.P5),
		P10:     time.Duration(s.LatNs.Percentile.P10),
		P20:     time.Duration(s.LatNs.Percentile.P20),
		P30:     time.Duration(s.LatNs.Percentile.P30),
		P40:     time.Duration(s.LatNs.Percentile.P40),
		P50:     time.Duration(s.LatNs.Percentile.P50),
		P60:     time.Duration(s.LatNs.Percentile.P60),
		P70:     time.Duration(s.LatNs.Percentile.P70),
		P80:     time.Duration(s.LatNs.Percentile.P80),
		P90:     time.Duration(s.LatNs.Percentile.P90),
		P95:     time.Duration(s.LatNs.Percentile.P95),
		P99:     time.Duration(s.LatNs.Percentile.P99),
		P995:    time.Duration(s.LatNs.Percentile.P995),
		P999:    time.Duration(s.LatNs.Percentile.P999),
		P9995:   time.Duration(s.LatNs.Percentile.P9995),
		P9999:   time.Duration(s.LatNs.Percentile.P9999),
	}
}

type FioNS struct {
	Min        int64         `json:"min,omitempty"`
	Max        int64         `json:"max,omitempty"`
	Mean       float32       `json:"mean,omitempty"`
	StdDev     float32       `json:"stddev,omitempty"`
	N          int64         `json:"N,omitempty"`
	Percentile FioPercentile `json:"percentile,omitempty"`
}

type FioDepth struct {
	FioDepth0    float32 `json:"0,omitempty"`
	FioDepth1    float32 `json:"1,omitempty"`
	FioDepth2    float32 `json:"2,omitempty"`
	FioDepth4    float32 `json:"4,omitempty"`
	FioDepth8    float32 `json:"8,omitempty"`
	FioDepth16   float32 `json:"16,omitempty"`
	FioDepth32   float32 `json:"32,omitempty"`
	FioDepth64   float32 `json:"64,omitempty"`
	FioDepthGE64 float32 `json:">=64,omitempty"`
}

type FioLatency struct {
	FioLat2      float32 `json:"2,omitempty"`
	FioLat4      float32 `json:"4,omitempty"`
	FioLat10     float32 `json:"10,omitempty"`
	FioLat20     float32 `json:"20,omitempty"`
	FioLat50     float32 `json:"50,omitempty"`
	FioLat100    float32 `json:"100,omitempty"`
	FioLat250    float32 `json:"250,omitempty"`
	FioLat500    float32 `json:"500,omitempty"`
	FioLat750    float32 `json:"750,omitempty"`
	FioLat1000   float32 `json:"1000,omitempty"`
	FioLat2000   float32 `json:"2000,omitempty"`
	FioLatGE2000 float32 `json:">=2000,omitempty"`
}

type FioDiskUtil struct {
	Name        string  `json:"name,omitempty"`
	ReadIos     int64   `json:"read_ios,omitempty"`
	WriteIos    int64   `json:"write_ios,omitempty"`
	ReadMerges  int64   `json:"read_merges,omitempty"`
	WriteMerges int64   `json:"write_merges,omitempty"`
	ReadTicks   int64   `json:"read_ticks,omitempty"`
	WriteTicks  int64   `json:"write_ticks,omitempty"`
	InQueue     int64   `json:"in_queue,omitempty"`
	Util        float32 `json:"util,omitempty"`
}

type FioPercentile struct {
	P1    int `json:"1.000000,omitempty"`
	P5    int `json:"5.000000,omitempty"`
	P10   int `json:"10.000000,omitempty"`
	P20   int `json:"20.000000,omitempty"`
	P30   int `json:"30.000000,omitempty"`
	P40   int `json:"40.000000,omitempty"`
	P50   int `json:"50.000000,omitempty"`
	P60   int `json:"60.000000,omitempty"`
	P70   int `json:"70.000000,omitempty"`
	P80   int `json:"80.000000,omitempty"`
	P90   int `json:"90.000000,omitempty"`
	P95   int `json:"95.000000,omitempty"`
	P99   int `json:"99.000000,omitempty"`
	P995  int `json:"99.500000,omitempty"`
	P999  int `json:"99.900000,omitempty"`
	P9995 int `json:"99.950000,omitempty"`
	P9999 int `json:"99.990000,omitempty"`
}

func (d FioDiskUtil) String() string {
	//Disk stats (read/write):
	//rbd4: ios=30022/11982, merge=0/313, ticks=1028675/1022768, in_queue=2063740, util=99.67%
	var du string
	du += fmt.Sprintf("  %s: ios=%d/%d merge=%d/%d ticks=%d/%d in_queue=%d, util=%f%%", d.Name, d.ReadIos,
		d.WriteIos, d.ReadMerges, d.WriteMerges, d.ReadTicks, d.WriteTicks, d.InQueue, d.Util)
	return du
}

func parseCollectorOptions(hostCollector *troubleshootv1beta2.FilesystemPerformance) ([]string, *FioJobOptions, error) {

	var operationSize uint64 = 1024
	if hostCollector.OperationSizeBytes > 0 {
		operationSize = hostCollector.OperationSizeBytes
	}
	var fileSize uint64 = 10 * 1024 * 1024
	if hostCollector.FileSize != "" {
		quantity, err := resource.ParseQuantity(hostCollector.FileSize)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "failed to parse fileSize %q", hostCollector.FileSize)
		}
		fileSizeInt64, ok := quantity.AsInt64()
		if !ok {
			return nil, nil, errors.Wrapf(err, "failed to parse fileSize %q", hostCollector.FileSize)
		}
		if fileSizeInt64 <= 0 {
			return nil, nil, errors.Wrapf(err, "fileSize %q must be greater than 0", hostCollector.FileSize)
		}
		fileSize = uint64(fileSizeInt64)
	}

	if hostCollector.Directory == "" {
		return nil, nil, errors.New("Directory is required to collect filesystem performance info")
	}

	runtime, err := getFioRuntime(hostCollector.RunTime)
	if err != nil {
		return nil, nil, err
	}

	latencyBenchmarkOptions := FioJobOptions{
		RW:        "write",
		IOEngine:  "sync",
		FDataSync: "1",
		Directory: hostCollector.Directory,
		Size:      strconv.FormatUint(fileSize, 10),
		BS:        strconv.FormatUint(operationSize, 10),
		Name:      FioJobName,
		RunTime:   runtime,
	}

	command := buildFioCommand(latencyBenchmarkOptions)

	return command, &latencyBenchmarkOptions, nil
}

// getFioRuntime returns the runTime value or the default if runTime is nil, empty or <= 0.
// This attepmts to maintain backwards compatibility prior to adding runTime to the collector spec,
// defaulting to 120 seconds.
func getFioRuntime(runTime *string) (string, error) {
	if runTime == nil {
		return DefaultFioRunTime, nil
	}
	if *runTime == "" {
		return "", nil // disable
	}
	i, err := strconv.Atoi(*runTime)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse runTime %q", *runTime)
	}
	if i <= 0 {
		return "", nil // disable
	}
	return *runTime, nil
}

func buildFioCommand(opts FioJobOptions) []string {
	command := []string{"fio"}
	v := reflect.ValueOf(opts)
	t := reflect.TypeOf(opts)
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)
		if !value.IsZero() {
			command = append(command, fmt.Sprintf("--%s=%v", strings.ToLower(field.Name), value.Interface()))
		}
	}
	command = append(command, "--output-format=json")
	return command
}

func collectFioResults(ctx context.Context, hostCollector *troubleshootv1beta2.FilesystemPerformance) (*FioResult, []byte, error) {

	command, opts, err := parseCollectorOptions(hostCollector)

	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to parse collector options")
	}

	klog.V(2).Infof("collecting fio results: %s", strings.Join(command, " "))

	// Capture both stdout and stderr
	cmd := exec.CommandContext(ctx, command[0], command[1:]...) // #nosec G204
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				return nil, stderr.Bytes(), errors.Wrapf(err, "fio failed; permission denied opening %s.  ensure this collector runs as root", opts.Directory)
			} else {
				return nil, stderr.Bytes(), errors.Wrapf(err, "fio failed with exit status %d", exitErr.ExitCode())
			}
		} else if e, ok := err.(*exec.Error); ok && e.Err == exec.ErrNotFound {
			return nil, stderr.Bytes(), errors.Wrapf(err, "command not found: %v. ensure fio is installed", command)
		} else {
			return nil, stderr.Bytes(), errors.Wrapf(err, "failed to run command: %v", command)
		}
	}

	var result FioResult
	err = json.Unmarshal(stdout.Bytes(), &result)
	if err != nil {
		return nil, stderr.Bytes(), errors.Wrap(err, "failed to unmarshal fio result")
	}

	return &result, stderr.Bytes(), nil
}

func (c *CollectHostFilesystemPerformance) RemoteCollect(progressChan chan<- interface{}) (map[string][]byte, error) {
	return nil, ErrRemoteCollectorNotImplemented
}
