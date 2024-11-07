package analyzer

import (
	"encoding/json"
	"testing"

	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetrieveCollectedContents(t *testing.T) {
	tests := []struct {
		name                     string
		getCollectedFileContents func(string) ([]byte, error) // Mock function
		localPath                string
		remoteNodeBaseDir        string
		remoteFileName           string
		expectedResult           []collectedContent
		expectedError            string
	}{
		{
			name: "successfully retrieve local content",
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == "localPath" {
					return []byte("localContent"), nil
				}
				return nil, &types.NotFoundError{Name: path}
			},
			localPath:         "localPath",
			remoteNodeBaseDir: "remoteBaseDir",
			remoteFileName:    "remoteFileName",
			expectedResult: []collectedContent{
				{
					NodeName: "",
					Data:     []byte("localContent"),
				},
			},
			expectedError: "",
		},
		{
			name: "local content not found, retrieve remote node content successfully",
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == constants.NODE_LIST_FILE {
					nodeNames := nodeNames{Nodes: []string{"node1", "node2"}}
					return json.Marshal(nodeNames)
				}
				if path == "remoteBaseDir/node1/remoteFileName" {
					return []byte("remoteContent1"), nil
				}
				if path == "remoteBaseDir/node2/remoteFileName" {
					return []byte("remoteContent2"), nil
				}
				return nil, &types.NotFoundError{Name: path}
			},
			localPath:         "localPath",
			remoteNodeBaseDir: "remoteBaseDir",
			remoteFileName:    "remoteFileName",
			expectedResult: []collectedContent{
				{
					NodeName: "node1",
					Data:     []byte("remoteContent1"),
				},
				{
					NodeName: "node2",
					Data:     []byte("remoteContent2"),
				},
			},
			expectedError: "",
		},
		{
			name: "fail to retrieve local content and node list",
			getCollectedFileContents: func(path string) ([]byte, error) {
				return nil, &types.NotFoundError{Name: path}
			},
			localPath:         "localPath",
			remoteNodeBaseDir: "remoteBaseDir",
			remoteFileName:    "remoteFileName",
			expectedResult:    nil,
			expectedError:     "",
		},
		{
			name: "fail to retrieve content for one of the nodes",
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == constants.NODE_LIST_FILE {
					nodeNames := nodeNames{Nodes: []string{"node1", "node2"}}
					return json.Marshal(nodeNames)
				}
				if path == "remoteBaseDir/node1/remoteFileName" {
					return []byte("remoteContent1"), nil
				}
				if path == "remoteBaseDir/node2/remoteFileName" {
					return nil, &types.NotFoundError{Name: path}
				}
				return nil, &types.NotFoundError{Name: path}
			},
			localPath:         "localPath",
			remoteNodeBaseDir: "remoteBaseDir",
			remoteFileName:    "remoteFileName",
			expectedResult:    nil,
			expectedError:     "failed to retrieve content for node node2",
		},
		{
			name: "fail to unmarshal node list",
			getCollectedFileContents: func(path string) ([]byte, error) {
				if path == constants.NODE_LIST_FILE {
					return []byte("invalidJSON"), nil
				}
				return nil, &types.NotFoundError{Name: path}
			},
			localPath:         "localPath",
			remoteNodeBaseDir: "remoteBaseDir",
			remoteFileName:    "remoteFileName",
			expectedResult:    nil,
			expectedError:     "failed to unmarshal node names",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := retrieveCollectedContents(
				test.getCollectedFileContents,
				test.localPath,
				test.remoteNodeBaseDir,
				test.remoteFileName,
			)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedResult, result)
			}
		})
	}
}
