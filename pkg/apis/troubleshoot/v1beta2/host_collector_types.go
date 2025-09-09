/*
Copyright 2019 Replicated, Inc..

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// HostCollectorSpec defines the desired state of HostCollector
type HostCollectorSpec struct {
	Collectors []*HostCollect `json:"collectors,omitempty" yaml:"collectors,omitempty"`
	Analyzers  []*HostAnalyze `json:"analyzers,omitempty" yaml:"analyzers,omitempty"`
	Uri        string         `json:"uri,omitempty" yaml:"uri,omitempty"`
}

// HostCollectorStatus defines the observed state of HostCollector
type HostCollectorStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HostCollector is the Schema for the collectors API
// +k8s:openapi-gen=true
type HostCollector struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   HostCollectorSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status HostCollectorStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HostCollectorList contains a list of Collector
type HostCollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostCollector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HostCollector{}, &HostCollectorList{})
}
