package cli

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/collect"
)

func showHostStdoutResults(format string, collectName string, results *collect.HostCollectResult) error {
	if format == "json" {
		return showHostStdoutResultsJSON(collectName, results.AllCollectedData)
	} else if format == "raw" {
		return showHostStdoutResultsRaw(collectName, results.AllCollectedData)
	}

	return errors.Errorf("unknown output format: %q", format)
}

func showRemoteStdoutResults(format string, collectName string, results *collect.RemoteCollectResult) error {
	if format == "json" {
		return showRemoteStdoutResultsJSON(collectName, results.AllCollectedData)
	}
	if format == "raw" {
		return errors.Errorf("raw format not supported for remote collectors")
	}
	return errors.Errorf("unknown output format: %q", format)
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
