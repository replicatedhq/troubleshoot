package v1beta1

type ClusterInfo struct {
}

type ClusterResources struct {
}

type Secret struct {
	Name         string `json:"name" yaml:"name"`
	Namespace    string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Key          string `json:"key,omitempty" yaml:"key,omitempty"`
	IncludeValue bool   `json:"includeValue,omitempty" yaml:"includeValue,omitempty"`
}

type Collect struct {
	ClusterInfo      *ClusterInfo      `json:"clusterInfo,omitempty" yaml:"clusterInfo,omitempty"`
	ClusterResources *ClusterResources `json:"clusterResources,omitempty" yaml:"clusterResources,omitempty"`
	Secret           *Secret           `json:"secret,omitempty" yaml:"secret,omitempty"`
}
