package constants

const (
	// DEFAULT_CLIENT_QPS indicates the maximum QPS from troubleshoot client.
	DEFAULT_CLIENT_QPS = 100
	// DEFAULT_CLIENT_QPS is maximum burst for throttle.
	DEFAULT_CLIENT_BURST = 100
	// DEFAULT_CLIENT_USER_AGENT is an field that specifies the caller of troubleshoot request.
	DEFAULT_CLIENT_USER_AGENT = "ReplicatedTroubleshoot"
	// VersionFilename is the name of the file that contains the support bundle version.
	VersionFilename = "version.yaml"
)
