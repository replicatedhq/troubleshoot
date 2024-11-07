package analyzer

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"
	"github.com/replicatedhq/troubleshoot/pkg/constants"
	"github.com/replicatedhq/troubleshoot/pkg/types"
)

type collectedContent struct {
	NodeName string
	Data     []byte
}

type nodeNames struct {
	Nodes []string `json:"nodes"`
}

func retrieveCollectedContents(
	getCollectedFileContents func(string) ([]byte, error),
	localPath string, remoteNodeBaseDir string, remoteFileName string,
) ([]collectedContent, error) {
	var collectedContents []collectedContent

	// Try to retrieve local data first
	if contents, err := getCollectedFileContents(localPath); err == nil {
		collectedContents = append(collectedContents, collectedContent{NodeName: "", Data: contents})
		// Return immediately if local content is available
		return collectedContents, nil
	}

	// Local data not available, move to remote collection
	nodeListContents, err := getCollectedFileContents(constants.NODE_LIST_FILE)
	if err != nil {
		if _, ok := err.(*types.NotFoundError); ok {
			return collectedContents, nil
		}
		return nil, errors.Wrap(err, "failed to get node list")
	}

	var nodeNames nodeNames
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

		collectedContents = append(collectedContents, collectedContent{NodeName: node, Data: nodeContents})
	}

	return collectedContents, nil
}
