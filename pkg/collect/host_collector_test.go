package collect

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type Params struct {
	Title        string
	ProgressChan chan error
}

// Mock data for testing
var testParams = RemoteCollectParams{
	Title:        "Test",
	ProgressChan: make(chan interface{}),
}

func Test_mapCollectorResultToOutput(t *testing.T) {
	result := map[string][]byte{
		"key1": []byte(`{"file1": "data1", "file2": "data2"}`),
		"key2": []byte(`{"file3": "data3"}`),
	}

	// Expected output after processing
	expectedCollectedData := map[string][]byte{
		"key1": []byte(`{"file1": "data1", "file2": "data2"}`),
		"key2": []byte(`{"file3": "data3"}`),
	}

	// Run the function logic
	allCollectedData := mapCollectorResultToOutput(result, testParams)

	// Validate the collected data
	for key, expected := range expectedCollectedData {
		assert.Equal(t, string(expected), string(allCollectedData[key]), "The collected data for key %s is incorrect", key)
	}
}
