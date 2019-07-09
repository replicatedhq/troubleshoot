package v1beta1

type ClusterInfo struct {
}

type ClusterResources struct {
}

type Collect struct {
	ClusterInfo      *ClusterInfo      `json:"cluster-info,omitempty" yaml:"cluster-info,omitempty"`
	ClusterResources *ClusterResources `json:"cluster-resources,omitempty" yaml:"cluster-resources,omitempty"`
}
