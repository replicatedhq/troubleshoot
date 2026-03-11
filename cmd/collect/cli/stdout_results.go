package cli

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

const (
	// FormatJSON is intended for CLI output.
	FormatJSON = "json"

	// FormatRaw is intended for consumption by a remote collector.  Output is a
	// string of quoted JSON.
	FormatRaw = "raw"
)

func showHostStdoutResults(format string, collectName string, results *collect.HostCollectResult) error {
	switch format {
	case FormatJSON:
		return showHostStdoutResultsJSON(collectName, results.AllCollectedData)
	case FormatRaw:
		return showHostStdoutResultsRaw(collectName, results.AllCollectedData)
	default:
		return errors.Errorf("unknown output format: %q", format)
	}
}

func showRemoteStdoutResults(format string, collectName string, results *collect.RemoteCollectResult) error {
	switch format {
	case FormatJSON:
		return showRemoteStdoutResultsJSON(collectName, results.AllCollectedData)
	case FormatRaw:
		return errors.Errorf("raw format not supported for remote collectors")
	default:
		return errors.Errorf("unknown output format: %q", format)
	}
}

func showHostStdoutResultsJSON(collectName string, results map[string][]byte) error {
	output := make(map[string]interface{})
	for file, collectorResult := range results {
		var collectedItems map[string]interface{}
		if err := json.Unmarshal([]byte(collectorResult), &collectedItems); err != nil {
			return errors.Wrap(err, "failed to marshal collector results")
		}
		output[file] = collectedItems
	}

	formatted, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to convert output to json")
	}

	fmt.Print(string(formatted))
	return nil
}

// showHostStdoutResultsRaw outputs the collector output as a string of quoted json.
func showHostStdoutResultsRaw(collectName string, results map[string][]byte) error {
	strData := map[string]string{}
	for k, v := range results {
		strData[k] = string(v)
	}
	formatted, err := json.MarshalIndent(strData, "", "    ")
	if err != nil {
		return errors.Wrap(err, "failed to convert output to json")
	}
	fmt.Print(string(formatted))
	return nil
}

func showRemoteStdoutResultsJSON(collectName string, results map[string][]byte) error {
	type CollectorResult map[string]interface{}
	type NodeResult map[string]CollectorResult

	var output = make(map[string]NodeResult)

	for node, result := range results {
		var nodeResult map[string]string
		if err := json.Unmarshal(result, &nodeResult); err != nil {
			return errors.Wrap(err, "failed to marshal node results")
		}
		nr := make(NodeResult)
		for file, collectorResult := range nodeResult {
			var collectedItems map[string]interface{}
			if err := json.Unmarshal([]byte(collectorResult), &collectedItems); err != nil {
				return errors.Wrap(err, "failed to marshal collector results")
			}
			nr[file] = collectedItems
		}
		output[node] = nr
	}

	formatted, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return errors.Wrap(err, "failed to convert output to json")
	}
	fmt.Print(string(formatted))
	return nil
}
