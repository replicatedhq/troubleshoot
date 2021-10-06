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

// HostPreflightSpec defines the desired state of HostPreflight
type HostPreflightSpec struct {
	Collectors       []*HostCollect   `json:"collectors,omitempty" yaml:"collectors,omitempty"`
	RemoteCollectors []*RemoteCollect `json:"remoteCollectors,omitempty" yaml:"remoteCollectors,omitempty"`
	Analyzers        []*HostAnalyze   `json:"analyzers,omitempty" yaml:"analyzers,omitempty"`
}

// HostPreflightStatus defines the observed state of HostPreflight
type HostPreflightStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HostPreflight is the Schema for the hostpreflights API
// +k8s:openapi-gen=true
type HostPreflight struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   HostPreflightSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status HostPreflightStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HostPreflightList contains a list of HostPreflight
type HostPreflightList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HostPreflight `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HostPreflight{}, &HostPreflightList{})
}
