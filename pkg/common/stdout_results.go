package common

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
)

const (
	// FormatJSON is intended for CLI output.
	FormatJSON = "json"

	// FormatRaw is intended for consumption by a remote collector.  Output is a
	// string of quoted JSON.
	FormatRaw = "raw"
)

func ShowRemoteStdoutResults(format string, collectName string, results *RemoteCollectResult) error {
	switch format {
	case FormatJSON:
		return ShowRemoteStdoutResultsJSON(collectName, results.AllCollectedData)
	case FormatRaw:
		return errors.Errorf("raw format not supported for remote collectors")
	default:
		return errors.Errorf("unknown output format: %q", format)
	}
}

func ShowRemoteStdoutResultsJSON(collectName string, results map[string][]byte) error {
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

func RemoteStdoutResultsJSON(collectName string, results map[string][]byte) (interface{}, error) {
	type CollectorResult map[string]interface{}
	type NodeResult map[string]CollectorResult

	var output = make(map[string]NodeResult)

	for node, result := range results {
		var nodeResult map[string]string
		if err := json.Unmarshal(result, &nodeResult); err != nil {
			return nil, errors.Wrap(err, "failed to marshal node results")
		}
		nr := make(NodeResult)
		for file, collectorResult := range nodeResult {
			var collectedItems map[string]interface{}
			if err := json.Unmarshal([]byte(collectorResult), &collectedItems); err != nil {
				return nil, errors.Wrap(err, "failed to marshal collector results")
			}
			nr[file] = collectedItems
		}
		output[node] = nr
	}

	return output, nil
}
