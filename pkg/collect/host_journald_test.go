package collect

import (
	"reflect"
	"testing"

	troubleshootv1beta2 "github.com/replicatedhq/troubleshoot/pkg/apis/troubleshoot/v1beta2"
)

func TestGenerateOptions(t *testing.T) {
	jd := &troubleshootv1beta2.HostJournald{
		System:  true,
		Dmesg:   true,
		Units:   []string{"unit1", "unit2"},
		Since:   "2022-01-01",
		Until:   "2022-01-31",
		Output:  "json",
		Lines:   100,
		Reverse: true,
		Utc:     true,
	}

	expectedOptions := []string{
		"--system",
		"--dmesg",
		"-u", "unit1",
		"-u", "unit2",
		"--since", "2022-01-01",
		"--until", "2022-01-31",
		"--output", "json",
		"-n", "100",
		"--reverse",
		"--utc",
	}

	options, err := generateOptions(jd)
	if err != nil {
		t.Fatalf("generateOptions failed with error: %v", err)
	}

	if !reflect.DeepEqual(options, expectedOptions) {
		t.Errorf("generateOptions returned incorrect options.\nExpected: %v\nActual: %v", expectedOptions, options)
	}
}
