package analyzer

import (
	"encoding/json"
	"fmt"
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
	"x86-64":    {"cmov", "cx8", "fpu", "fxsr", "mmx", "syscall", "sse", "sse2"},
	"x86-64-v2": {"cx16", "lahf_lm", "popcnt", "ssse3", "sse4_1", "sse4_2", "ssse3"},
	"x86-64-v3": {"avx", "avx2", "bmi1", "bmi2", "f16c", "fma", "lzcnt", "movbe", "xsave"},
	"x86-64-v4": {"avx512f", "avx512bw", "avx512cd", "avx512dq", "avx512vl"},
}

type AnalyzeHostCPU struct {
	hostAnalyzer *troubleshootv1beta2.CPUAnalyze
}

func (a *AnalyzeHostCPU) Title() string {
	return hostAnalyzerTitleOrDefault(a.hostAnalyzer.AnalyzeMeta, "Number of CPUs")
}

func (a *AnalyzeHostCPU) IsExcluded() (bool, error) {
	return isExcluded(a.hostAnalyzer.Exclude)
}

func (a *AnalyzeHostCPU) CheckCondition(when string, data []byte) (bool, error) {

	cpuInfo := collect.CPUInfo{}
	if err := json.Unmarshal(data, &cpuInfo); err != nil {
		return false, fmt.Errorf("failed to unmarshal data into CPUInfo: %v", err)
	}

	return compareHostCPUConditionalToActual(when, cpuInfo.LogicalCount, cpuInfo.PhysicalCount, cpuInfo.Flags, cpuInfo.MachineArch)

}

func (a *AnalyzeHostCPU) Analyze(
	getCollectedFileContents func(string) ([]byte, error), findFiles getChildCollectedFileContents,
) ([]*AnalyzeResult, error) {
	result := AnalyzeResult{Title: a.Title()}

	// Use the generic function to collect both local and remote data
	collectedContents, err := retrieveCollectedContents(
		getCollectedFileContents,
		collect.HostCPUPath,     // Local path
		collect.NodeInfoBaseDir, // Remote base directory
		collect.HostCPUFileName, // Remote file name
	)
	if err != nil {
		return []*AnalyzeResult{&result}, err
	}

	results, err := analyzeHostCollectorResults(collectedContents, a.hostAnalyzer.Outcomes, a.CheckCondition, a.Title())
	if err != nil {
		return nil, errors.Wrap(err, "failed to analyze CPU info")
	}

	// Add template support
	content := collectedContents[0].Data
	cpuInfo := collect.CPUInfo{}
	if err := json.Unmarshal(content, &cpuInfo); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal cpu info")
	}

	for _, r := range results {
		r.Message = strings.ReplaceAll(r.Message, "{{ Info.MachineArch }}", cpuInfo.MachineArch)
	}

	return results, nil
}

func doCompareHostCPUMicroArchitecture(microarch string, flags []string) (res bool, err error) {
	specifics := make([]string, 0)
	switch microarch {
	case "x86-64-v4":
		specifics = append(specifics, microarchs["x86-64-v4"]...)
		fallthrough
	case "x86-64-v3":
		specifics = append(specifics, microarchs["x86-64-v3"]...)
		fallthrough
	case "x86-64-v2":
		specifics = append(specifics, microarchs["x86-64-v2"]...)
		fallthrough
	case "x86-64":
		specifics = append(specifics, microarchs["x86-64"]...)
	default:
		return false, errors.Errorf("troubleshoot does not yet support microarchitecture %q", microarch)
	}

	for _, flag := range specifics {
		if slices.Contains(flags, flag) {
			continue
		}
		return false, nil
	}
	return true, nil
}

func doCompareHostCPUFlags(expected string, flags []string) (res bool, err error) {
	expectedFlags := strings.Split(expected, ",")
	if len(expectedFlags) == 0 {
		return false, errors.New("expected flags cannot be empty")
	}
	for _, flag := range expectedFlags {
		if slices.Contains(flags, flag) {
			continue
		}
		return false, nil
	}
	return true, nil
}

func compareHostCPUConditionalToActual(conditional string, logicalCount int, physicalCount int, flags []string, machineArch string) (res bool, err error) {
	compareLogical := false
	comparePhysical := false
	compareUnspecified := false
	compareMachineArch := false

	comparator := ""
	desired := ""

	/* When the conditional is in the format of "logical <comparator> <desired>"
	   example: when: "count < 2"
	*/

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
		} else if strings.ToLower(parts[0]) == "machinearch" {
			compareMachineArch = true
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

	// hasFlags allows users to query for specific flags on the CPU.
	if strings.ToLower(comparator) == "hasflags" {
		return doCompareHostCPUFlags(desired, flags)
	}

	if !compareLogical && !comparePhysical && !compareUnspecified && !compareMachineArch {
		return false, errors.New("unable to parse conditional")
	}

	if compareLogical {
		return doCompareHostCPU(comparator, desired, logicalCount)
	} else if comparePhysical {
		return doCompareHostCPU(comparator, desired, physicalCount)
	} else if compareMachineArch {
		return doCompareMachineArch(comparator, desired, machineArch)
	} else {
		actual := logicalCount
		if physicalCount > logicalCount {
			actual = physicalCount
		}

		return doCompareHostCPU(comparator, desired, actual)
	}
}

func doCompareMachineArch(operator string, desired string, actual string) (bool, error) {
	switch operator {
	case "=", "==", "===":
		return actual == desired, nil
	case "!=", "!==":
		return actual != desired, nil
	}
	return false, errors.New("unknown operator")
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
	case "!=", "!==":
		return actual != int(desiredInt), nil
	}

	return false, errors.New("unknown operator")
}
