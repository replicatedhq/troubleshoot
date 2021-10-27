package analyzer

import (
	_ "embed"
)

//go:embed files/deployments.json
var collectedDeployments string

//go:embed files/nodes.json
var collectedNodes string

//go:embed files/jobs.json
var collectedJobs string

//go:embed files/replicasets.json
var collectedReplicaSets string
