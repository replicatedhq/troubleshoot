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
	// MAX_TIME_TO_WAIT_FOR_POD_DELETION is the maximum time to wait for pod deletion.
	// 0 seconds for force deletion.
	MAX_TIME_TO_WAIT_FOR_POD_DELETION = 60 * time.Second
	// Tracing constants
	LIB_TRACER_NAME             = "github.com/replicatedhq/troubleshoot"
	TROUBLESHOOT_ROOT_SPAN_NAME = "ReplicatedTroubleshootRootSpan"
	EXCLUDED                    = "excluded"
	ANALYSIS_FILENAME           = "analysis.json"

	// Cluster Resources Collector Directories
	CLUSTER_RESOURCES_DIR                          = "cluster-resources"
	CLUSTER_RESOURCES_NAMESPACES                   = "namespaces"
	CLUSTER_RESOURCES_AUTH_CANI                    = "auth-cani-list"
	CLUSTER_RESOURCES_PODS                         = "pods"
	CLUSTER_RESOURCES_PODS_LOGS                    = "pods/logs"
	CLUSTER_RESOURCES_POD_DISRUPTION_BUDGETS       = "pod-disruption-budgets"
	CLUSTER_RESOURCES_SERVICES                     = "services"
	CLUSTER_RESOURCES_DEPLOYMENTS                  = "deployments"
	CLUSTER_RESOURCES_REPLICASETS                  = "replicasets"
	CLUSTER_RESOURCES_STATEFULSETS                 = "statefulsets"
	CLUSTER_RESOURCES_DAEMONSETS                   = "daemonsets"
	CLUSTER_RESOURCES_JOBS                         = "jobs"
	CLUSTER_RESOURCES_CRONJOBS                     = "cronjobs"
	CLUSTER_RESOURCES_INGRESS                      = "ingress"
	CLUSTER_RESOURCES_NETWORK_POLICY               = "network-policy"
	CLUSTER_RESOURCES_RESOURCE_QUOTA               = "resource-quota"
	CLUSTER_RESOURCES_STORAGE_CLASS                = "storage-classes"
	CLUSTER_RESOURCES_INGRESS_CLASS                = "ingress-classes"
	CLUSTER_RESOURCES_CUSTOM_RESOURCE_DEFINITIONS  = "custom-resource-definitions"
	CLUSTER_RESOURCES_CUSTOM_RESOURCES             = "custom-resources"
	CLUSTER_RESOURCES_IMAGE_PULL_SECRETS           = "image-pull-secrets" // nolint:gosec
	CLUSTER_RESOURCES_NODES                        = "nodes"
	CLUSTER_RESOURCES_GROUPS                       = "groups"
	CLUSTER_RESOURCES_RESOURCES                    = "resources"
	CLUSTER_RESOURCES_LIMITRANGES                  = "limitranges"
	CLUSTER_RESOURCES_EVENTS                       = "events"
	CLUSTER_RESOURCES_PVS                          = "pvs"
	CLUSTER_RESOURCES_PVCS                         = "pvcs"
	CLUSTER_RESOURCES_ROLES                        = "roles"
	CLUSTER_RESOURCES_ROLE_BINDINGS                = "rolebindings"
	CLUSTER_RESOURCES_CLUSTER_ROLES                = "clusterroles"
	CLUSTER_RESOURCES_CLUSTER_ROLE_BINDINGS        = "clusterrolebindings"
	CLUSTER_RESOURCES_PRIORITY_CLASS               = "priorityclasses"
	CLUSTER_RESOURCES_ENDPOINTS                    = "endpoints"
	CLUSTER_RESOURCES_ENDPOINTSLICES               = "endpointslices"
	CLUSTER_RESOURCES_SERVICE_ACCOUNTS             = "serviceaccounts"
	CLUSTER_RESOURCES_LEASES                       = "leases"
	CLUSTER_RESOURCES_VOLUME_ATTACHMENTS           = "volumeattachments"
	CLUSTER_RESOURCES_CONFIGMAPS                   = "configmaps"
	CLUSTER_RESOURCES_REPLICATED_LICENSE           = "license.json"
	CLUSTER_RESOURCES_CERTIFICATE_SIGNING_REQUESTS = "certificatesigningrequests"

	// SelfSubjectRulesReview evaluation responses
	SELFSUBJECTRULESREVIEW_ERROR_AUTHORIZATION_WEBHOOK_UNSUPPORTED = "webhook authorizer does not support user rule resolution"

	// Custom exit codes
	EXIT_CODE_CATCH_ALL   = 1
	EXIT_CODE_SPEC_ISSUES = 2
	EXIT_CODE_FAIL        = 3
	EXIT_CODE_WARN        = 4

	// Troubleshoot label constants
	TroubleshootIOLabelKey = "troubleshoot.io/kind"
	TroubleshootSHLabelKey = "troubleshoot.sh/kind"
	SupportBundleKey       = "support-bundle-spec"
	RedactorKey            = "redactor-spec"
	PreflightKey           = "preflight.yaml"
	PreflightKey2          = "preflight-spec"

	// Troubleshoot spec constants
	Troubleshootv1beta3Kind = "troubleshoot.sh/v1beta3"
	Troubleshootv1beta2Kind = "troubleshoot.sh/v1beta2"
	Troubleshootv1beta1Kind = "troubleshoot.replicated.com/v1beta1"

	// TermUI Display Constants
	MESSAGE_TEXT_PADDING                = 4
	MESSAGE_TEXT_LINES_MARGIN_TO_BOTTOM = 4

	// This is the initial size of the buffer allocated.
	// Under the hood, an array of size N is allocated in memory
	BUF_INIT_SIZE = 4096 // 4KB

	// This is the muximum size the buffer can grow to
	// Its not what the buffer will be allocated to initially
	SCANNER_MAX_SIZE = 10 * 1024 * 1024 // 10MB

	// Goldpinger constants
	GP_CHECK_ALL_RESULTS_PATH = "goldpinger/check_all.json"

	// GP_DEFAULT_IMAGE is the default image used for goldpinger
	// "replicated/kurl-util" would be better
	// since its always in airgap envs, but its tagged
	// with the kurl versions which would not work since they
	// are not always the same
	GP_DEFAULT_IMAGE     = "alpine:3"
	GP_DEFAULT_NAMESPACE = "default"

	// Analyzer Outcome types
	OUTCOME_PASS = "pass"
	OUTCOME_WARN = "warn"
	OUTCOME_FAIL = "fail"

	// List of remote nodes to collect data from in a support bundle
	NODE_LIST_FILE = "host-collectors/system/node_list.json"
)
