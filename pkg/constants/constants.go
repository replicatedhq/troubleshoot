package constants

const (
	// DEFAULT_CLIENT_QPS indicates the maximum QPS from troubleshoot client.
	DEFAULT_CLIENT_QPS = 100
	// DEFAULT_CLIENT_QPS is maximum burst for throttle.
	DEFAULT_CLIENT_BURST = 100
	// DEFAULT_CLIENT_USER_AGENT is an field that specifies the caller of troubleshoot request.
	DEFAULT_CLIENT_USER_AGENT = "ReplicatedTroubleshoot"
	// DEFAULT_MAX_NB_CONCURRENT_COLLECTORS specifies the maximum number of goroutines to run at once during collection.
	DEFAULT_MAX_NB_CONCURRENT_COLLECTORS = 5
)
