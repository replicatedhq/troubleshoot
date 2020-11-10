package analyzer

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/rrd"
)

type CollectdSummary struct {
	Load float64
}

func analyzeCollectd(analyzer *troubleshootv1beta2.CollectdAnalyze, getCollectedFileContents func(string) (map[string][]byte, error)) (*AnalyzeResult, error) {
	rrdArchives, err := getCollectedFileContents("/collectd/rrd/*.tar")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find rrd archives")
	}

	tmpDir, err := ioutil.TempDir("", "rrd")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp rrd dir")
	}
	defer os.RemoveAll(tmpDir)

	for name, data := range rrdArchives {
		destDir := filepath.Join(tmpDir, filepath.Base(name))
		if err := extractRRDFiles(data, destDir); err != nil {
			return nil, errors.Wrap(err, "failed to extract rrd file")
		}
	}

	loadFiles, err := findRRDLoadFiles(tmpDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find load files")
	}

	collectdSummary := CollectdSummary{
		Load: 0,
	}

	// load files are always present, so this loop can be used for all host metrics
	for _, loadFile := range loadFiles {
		pathParts := strings.Split(loadFile, string(filepath.Separator))
		if len(pathParts) < 3 {
			continue
		}

		// .../<hostname>/load/load.rrd
		hostname := pathParts[len(pathParts)-3]
		hostDir := strings.TrimSuffix(loadFile, "/load/load.rrd")

		hostLoad, err := getHostLoad(analyzer, loadFile, hostDir)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to find analyze %s files", hostname)
		}

		collectdSummary.Load = math.Max(collectdSummary.Load, hostLoad)
	}

	result, err := getCollectdAnalyzerOutcome(analyzer, collectdSummary)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate outcome")
	}

	return result, nil
}

func extractRRDFiles(archiveData []byte, dst string) error {
	tarReader := tar.NewReader(bytes.NewReader(archiveData))
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return errors.Wrap(err, "failed to read rrd archive")
		}

		if header.Typeflag != tar.TypeReg {
			continue
		}

		dstFileName := filepath.Join(dst, header.Name)

		if err := os.MkdirAll(filepath.Dir(dstFileName), 0755); err != nil {
			return errors.Wrap(err, "failed to create dest path")
		}

		err = func() error {
			f, err := os.Create(dstFileName)
			if err != nil {
				return errors.Wrap(err, "failed to create dest file")
			}
			defer f.Close()

			_, err = io.Copy(f, tarReader)
			if err != nil {
				return errors.Wrap(err, "failed to copy")
			}
			return nil
		}()

		if err != nil {
			return errors.Wrap(err, "failed to write dest file")
		}
	}
}

func findRRDLoadFiles(rootDir string) ([]string, error) {
	files := make([]string, 0)
	err := filepath.Walk(rootDir, func(filename string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if filepath.Base(filename) == "load.rrd" {
			files = append(files, filename)
		}

		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to find rrd load files")
	}

	return files, nil
}

func getHostLoad(analyzer *troubleshootv1beta2.CollectdAnalyze, loadFile string, hostRoot string) (float64, error) {
	numberOfCPUs := 0
	err := filepath.Walk(hostRoot, func(filename string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}

		if strings.HasPrefix(filepath.Base(filename), "cpu-") {
			numberOfCPUs++
		}
		return nil
	})
	if err != nil {
		return 0, errors.Wrap(err, "failed to find rrd files")
	}

	if numberOfCPUs == 0 {
		numberOfCPUs = 1 // what else can we do here?  return an error?
	}

	fileInfo, err := rrd.Info(loadFile)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get rrd info")
	}

	// Query RRD data.  Start and end have to be multiples of step.

	window := 7 * 24 * time.Hour
	step := 1 * time.Hour
	lastUpdate := int64(fileInfo["last_update"].(uint))
	endSeconds := int64(lastUpdate/int64(step.Seconds())) * int64(step.Seconds())
	end := time.Unix(int64(endSeconds), 0)
	start := end.Add(-window)
	fetchResult, err := rrd.Fetch(loadFile, "MAX", start, end, step)
	if err != nil {
		return 0, errors.Wrap(err, "failed to fetch load data")
	}
	defer fetchResult.FreeValues()

	values := fetchResult.Values()
	maxLoad := float64(0)
	for i := 0; i < len(values); i += 3 { // +3 because "shortterm", "midterm", "longterm"
		v := values[i+1] // midterm
		if math.IsNaN(v) {
			continue
		}
		maxLoad = math.Max(maxLoad, values[i+1])
	}

	return maxLoad / float64(numberOfCPUs), nil
}

func getCollectdAnalyzerOutcome(analyzer *troubleshootv1beta2.CollectdAnalyze, collectdSummary CollectdSummary) (*AnalyzeResult, error) {
	collectorName := analyzer.CollectorName
	if collectorName == "" {
		collectorName = "rrd"
	}

	title := analyzer.CheckName
	if title == "" {
		title = collectorName
	}
	result := &AnalyzeResult{
		Title:   title,
		IconKey: "host_load_analyze",
		IconURI: "https://troubleshoot.sh/images/analyzer-icons/rrd-analyze.svg",
	}

	for _, outcome := range analyzer.Outcomes {
		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}

			isMatch, err := compareCollectdConditionalToActual(outcome.Fail.When, collectdSummary)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare rrd fail conditional")
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return result, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Pass.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}

			isMatch, err := compareCollectdConditionalToActual(outcome.Warn.When, collectdSummary)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare rrd warn conditional")
			}

			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return result, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}

			isMatch, err := compareCollectdConditionalToActual(outcome.Pass.When, collectdSummary)
			if err != nil {
				return result, errors.Wrap(err, "failed to compare rrd pass conditional")
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return result, nil
			}
		}
	}

	return result, nil
}

func compareCollectdConditionalToActual(conditional string, collectdSummary CollectdSummary) (bool, error) {
	parts := strings.Split(strings.TrimSpace(conditional), " ")

	if len(parts) != 3 {
		return false, errors.New("unable to parse conditional")
	}

	switch parts[0] {
	case "load":
		expected, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return false, errors.Wrap(err, "failed to parse float")
		}

		switch parts[1] {
		case "=", "==", "===":
			return collectdSummary.Load == expected, nil
		case "!=", "!==":
			return collectdSummary.Load != expected, nil
		case "<":
			return collectdSummary.Load < expected, nil

		case ">":
			return collectdSummary.Load > expected, nil

		case "<=":
			return collectdSummary.Load <= expected, nil

		case ">=":
			return collectdSummary.Load >= expected, nil
		}

		return false, errors.Errorf("unknown rrd comparator: %q", parts[0])
	}

	return false, nil
}
