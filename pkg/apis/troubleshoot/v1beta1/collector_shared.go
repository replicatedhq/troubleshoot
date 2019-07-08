package v1beta1

type ClusterInfo struct {
}

type Collect struct {
	ClusterInfo *ClusterInfo `json:"cluster-info,omitempty"`
}
