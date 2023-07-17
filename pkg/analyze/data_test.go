package analyzer

import (
	_ "embed"
)

//go:embed files/deployments/default.json
var defaultDeployments string

//go:embed files/deployments/monitoring.json
var monitoringDeployments string

//go:embed files/deployments/kube-system.json
var kubeSystemDeployments string

//go:embed files/nodes.json
var collectedNodes string

//go:embed files/jobs/test.json
var testJobs string

//go:embed files/jobs/projectcontour.json
var projectcontourJobs string

//go:embed files/replicasets/default.json
var defaultReplicaSets string

//go:embed files/replicasets/rook-ceph.json
var rookCephReplicaSets string

//go:embed files/statefulsets/default.json
var defaultStatefulSets string

//go:embed files/statefulsets/monitoring.json
var monitoringStatefulSets string

//go:embed files/pods/default.json
var defaultPods string

//go:embed files/pods/other.json
var otherPods string

//go:embed files/pods/default-unhealthy.json
var defaultPodsUnhealthy string

//go:embed files/pods/other-unhealthy.json
var otherPodsUnhealthy string

//go:embed files/pods/message-pending-node-affinity.json
var messagePendingNodeAffinity string

//go:embed files/pods/message-container-creating-failed-mount.json
var messageContainerCreatingFailedMount string

//go:embed files/events/message-container-creating-failed-mount.json
var messageContainerCreatingFailedMountEvents string

//go:embed files/pods/message-pod-crashloop-backoff.json
var messagePodCrashLoopBackOff string

//go:embed files/pods/message-pod-init-crashloop-backoff.json
var messagePodInitCrashLoopBackOff string

//go:embed files/events/message-pod-init-crashloop-backoff.json
var messagePodInitCrashLoopBackOffEvents string

//go:embed files/pods/message-pending-pod-resources.json
var messagePendingPodResources string

//go:embed files/pods/message-oomkill-pod.json
var messageOOMKillPod string

//go:embed files/pods/message-image-pull-fail.json
var messageImagePullFail string

//go:embed files/events/message-image-pull-fail.json
var messageImagePullFailEvents string
