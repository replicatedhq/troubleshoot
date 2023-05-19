package constants

import "time"

const (
	// DEFAULT_CLIENT_QPS indicates the maximum QPS from troubleshoot client.
	DEFAULT_CLIENT_QPS = 100
	// DEFAULT_CLIENT_QPS is maximum burst for throttle.
	DEFAULT_CLIENT_BURST = 100
	// DEFAULT_CLIENT_USER_AGENT is an field that specifies the caller of troubleshoot request.
	DEFAULT_CLIENT_USER_AGENT = "ReplicatedTroubleshoot"
	// VERSION_FILENAME is the name of the file that contains the support bundle version.
	VERSION_FILENAME = "version.yaml"
	// DEFAULT_LOGS_COLLECTOR_TIMEOUT is the default timeout for logs collector.
	DEFAULT_LOGS_COLLECTOR_TIMEOUT = 60 * time.Second

	// Tracing constants

	LIB_TRACER_NAME             = "github.com/replicatedhq/troubleshoot"
	TROUBLESHOOT_ROOT_SPAN_NAME = "ReplicatedTroubleshootRootSpan"
	EXCLUDED                    = "excluded"

	// Cluster Resources Collector Directories
	CLUSTER_RESOURCES_DIR                         = "cluster-resources"
	CLUSTER_RESOURCES_NAMESPACES                  = "namespaces"
	CLUSTER_RESOURCES_AUTH_CANI                   = "auth-cani-list"
	CLUSTER_RESOURCES_PODS                        = "pods"
	CLUSTER_RESOURCES_PODS_LOGS                   = "pods/logs"
	CLUSTER_RESOURCES_POD_DISRUPTION_BUDGETS      = "pod-disruption-budgets"
	CLUSTER_RESOURCES_SERVICES                    = "services"
	CLUSTER_RESOURCES_DEPLOYMENTS                 = "deployments"
	CLUSTER_RESOURCES_REPLICASETS                 = "replicasets"
	CLUSTER_RESOURCES_STATEFULSETS                = "statefulsets"
	CLUSTER_RESOURCES_JOBS                        = "jobs"
	CLUSTER_RESOURCES_CRONJOBS                    = "cronjobs"
	CLUSTER_RESOURCES_INGRESS                     = "ingress"
	CLUSTER_RESOURCES_NETWORK_POLICY              = "network-policy"
	CLUSTER_RESOURCES_RESOURCE_QUOTA              = "resource-quota"
	CLUSTER_RESOURCES_STORAGE_CLASS               = "storage-classes"
	CLUSTER_RESOURCES_CUSTOM_RESOURCE_DEFINITIONS = "custom-resource-definitions"
	CLUSTER_RESOURCES_CUSTOM_RESOURCES            = "custom-resources"
	CLUSTER_RESOURCES_IMAGE_PULL_SECRETS          = "image-pull-secrets" // nolint:gosec
	CLUSTER_RESOURCES_NODES                       = "nodes"
	CLUSTER_RESOURCES_GROUPS                      = "groups"
	CLUSTER_RESOURCES_RESOURCES                   = "resources"
	CLUSTER_RESOURCES_LIMITRANGES                 = "limitranges"
	CLUSTER_RESOURCES_EVENTS                      = "events"
	CLUSTER_RESOURCES_PVS                         = "pvs"
	CLUSTER_RESOURCES_PVCS                        = "pvcs"
	CLUSTER_RESOURCES_ROLES                       = "roles"
	CLUSTER_RESOURCES_ROLE_BINDINGS               = "rolebindings"
	CLUSTER_RESOURCES_CLUSTER_ROLES               = "clusterroles"
	CLUSTER_RESOURCES_CLUSTER_ROLE_BINDINGS       = "clusterrolebindings"
	CLUSTER_RESOURCES_PRIORITY_CLASS              = "priorityclasses"

	// Custom exit codes
	EXIT_CODE_CATCH_ALL   = 1
	EXIT_CODE_SPEC_ISSUES = 2
	EXIT_CODE_FAIL        = 3
	EXIT_CODE_WARN        = 4
)
