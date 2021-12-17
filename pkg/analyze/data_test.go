package analyzer

import (
	_ "embed"
)

//go:embed files/deployments/default.json
var defaultDeployments string

//go:embed files/nodes.json
var collectedNodes string

//go:embed files/jobs/test.json
var testJobs string

//go:embed files/replicasets/rook-ceph.json
var rookCephReplicaSets string
