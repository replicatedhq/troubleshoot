package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
)

type CollectedContent struct {
	NodeName string
	Data     CollectorData
}

type CollectorData interface{}

type NodeNames struct {
	Nodes []string `json:"nodes"`
}

func retrieveCollectedContents(
	getCollectedFileContents func(string) ([]byte, error),
	localPath string, remoteNodeBaseDir string, remoteFileName string,
) ([]CollectedContent, error) {
	var collectedContents []CollectedContent

	// Try to retrieve local data first
	if contents, err := getCollectedFileContents(localPath); err == nil {
		collectedContents = append(collectedContents, CollectedContent{NodeName: "", Data: contents})
		// Return immediately if local content is available
		return collectedContents, nil
	}

	// Local data not available, move to remote collection
	nodeListContents, err := getCollectedFileContents(constants.NODE_LIST_FILE)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get node list")
	}

	var nodeNames NodeNames
	if err := json.Unmarshal(nodeListContents, &nodeNames); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal node names")
	}

	// Collect data for each node
	for _, node := range nodeNames.Nodes {
		nodeFilePath := fmt.Sprintf("%s/%s/%s", remoteNodeBaseDir, node, remoteFileName)
		nodeContents, err := getCollectedFileContents(nodeFilePath)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to retrieve content for node %s", node)
		}

		collectedContents = append(collectedContents, CollectedContent{NodeName: node, Data: nodeContents})
	}

	return collectedContents, nil
}
