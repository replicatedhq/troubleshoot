package analyzer

import (
	"encoding/json"
	"slices"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

// microarchs holds a list of features present in each microarchitecture.
// ref: https://gitlab.com/x86-psABIs/x86-64-ABI
// ref: https://developers.redhat.com/blog/2021/01/05/building-red-hat-enterprise-linux-9-for-the-x86-64-v2-microarchitecture-level
var microarchs = map[string][]string{
	"x86-64-v2": {"cx16", "lahf_lm", "popcnt", "ssse3", "sse4_1", "sse4_2", "ssse3"},
	"x86-64-v3": {"avx", "avx2", "bmi1", "bmi2", "f16c", "fma", "lzcnt", "movbe", "xsave"},
	"x86-64-v4": {"avx512f", "avx512bw", "avx512cd", "avx512dq", "avx512vl"},
}

// x8664BaseFeatures are the features that are present in all x86-64 microarchitectures.
var x8664BaseFeatures = []string{"cmov", "cx8", "fpu", "fxsr", "mmx", "syscall", "sse", "sse2"}

type AnalyzeHostCPU struct {
	hostAnalyzer *troubleshootv1beta2.CPUAnalyze
}

func (a *AnalyzeHostCPU) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Number of CPUs")
}

func (a *AnalyzeHostCPU) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostCPU) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	hostAnalyzer := a.hostAnalyzer

	contents, err := getCollectedFileContents(collect.HostCPUPath)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get collected file")
	}

	cpuInfo := collect.CPUInfo{}
	if err := json.Unmarshal(contents, &cpuInfo); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal cpu info")
	}

	result := AnalyzeResult{
		Title: a.Title(),
	}

	for _, outcome := range hostAnalyzer.Outcomes {

		if outcome.Fail != nil {
			if outcome.Fail.When == "" {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return []*AnalyzeResult{&result}, nil
			}

			isMatch, err := compareHostCPUConditionalToActual(outcome.Fail.When, cpuInfo.LogicalCount, cpuInfo.PhysicalCount, cpuInfo.Flags)
			if err != nil {
				return nil, errors.Wrap(err, "failed to compare")
			}

			if isMatch {
				result.IsFail = true
				result.Message = outcome.Fail.Message
				result.URI = outcome.Fail.URI

				return []*AnalyzeResult{&result}, nil
			}
		} else if outcome.Warn != nil {
			if outcome.Warn.When == "" {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return []*AnalyzeResult{&result}, nil
			}

			isMatch, err := compareHostCPUConditionalToActual(outcome.Warn.When, cpuInfo.LogicalCount, cpuInfo.PhysicalCount, cpuInfo.Flags)
			if err != nil {
				return nil, errors.Wrap(err, "failed to compare")
			}

			if isMatch {
				result.IsWarn = true
				result.Message = outcome.Warn.Message
				result.URI = outcome.Warn.URI

				return []*AnalyzeResult{&result}, nil
			}
		} else if outcome.Pass != nil {
			if outcome.Pass.When == "" {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return []*AnalyzeResult{&result}, nil
			}

			isMatch, err := compareHostCPUConditionalToActual(outcome.Pass.When, cpuInfo.LogicalCount, cpuInfo.PhysicalCount, cpuInfo.Flags)
			if err != nil {
				return nil, errors.Wrap(err, "failed to compare")
			}

			if isMatch {
				result.IsPass = true
				result.Message = outcome.Pass.Message
				result.URI = outcome.Pass.URI

				return []*AnalyzeResult{&result}, nil
			}
		}
	}

	return []*AnalyzeResult{&result}, nil
}

func doCompareHostCPUMicroArchitecture(microarch string, flags []string) (res bool, err error) {
	specifics, ok := microarchs[microarch]
	if !ok && microarch != "x86-64" {
		return false, errors.Errorf("troubleshoot does not yet support microarchitecture %q", microarch)
	}
	expectedFlags := x8664BaseFeatures
	if len(specifics) > 0 {
		expectedFlags = append(expectedFlags, specifics...)
	}
	for _, flag := range expectedFlags {
		if slices.Contains(flags, flag) {
			continue
		}
		return false, nil
	}
	return true, nil
}

func compareHostCPUConditionalToActual(conditional string, logicalCount int, physicalCount int, flags []string) (res bool, err error) {
	compareLogical := false
	comparePhysical := false
	compareUnspecified := false

	comparator := ""
	desired := ""

	parts := strings.Split(conditional, " ")
	if len(parts) == 3 {
		comparator = parts[1]
		desired = parts[2]
		if strings.ToLower(parts[0]) == "logical" {
			compareLogical = true
		} else if strings.ToLower(parts[0]) == "physical" {
			comparePhysical = true
		} else if strings.ToLower(parts[0]) == "count" {
			compareUnspecified = true
		}
	} else if len(parts) == 2 {
		compareUnspecified = true
		comparator = parts[0]
		desired = parts[1]
	}

	// analyze if the cpu supports a specific set of features, aka as micrarchitecture.
	if strings.ToLower(comparator) == "supports" {
		return doCompareHostCPUMicroArchitecture(desired, flags)
	}

	if !compareLogical && !comparePhysical && !compareUnspecified {
		return false, errors.New("unable to parse conditional")
	}

	if compareLogical {
		return doCompareHostCPU(comparator, desired, logicalCount)
	} else if comparePhysical {
		return doCompareHostCPU(comparator, desired, physicalCount)
	} else {
		actual := logicalCount
		if physicalCount > logicalCount {
			actual = physicalCount
		}

		return doCompareHostCPU(comparator, desired, actual)
	}
}

func doCompareHostCPU(operator string, desired string, actual int) (bool, error) {
	desiredInt, err := strconv.ParseInt(desired, 10, 64)
	if err != nil {
		return false, errors.Wrap(err, "failed to parse")
	}

	switch operator {
	case "<":
		return actual < int(desiredInt), nil
	case "<=":
		return actual <= int(desiredInt), nil
	case ">":
		return actual > int(desiredInt), nil
	case ">=":
		return actual >= int(desiredInt), nil
	case "=", "==", "===":
		return actual == int(desiredInt), nil
	}

	return false, errors.New("unknown operator")
}
