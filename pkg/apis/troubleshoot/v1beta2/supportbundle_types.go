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

// SupportBundleSpec defines the desired state of SupportBundle
type SupportBundleSpec struct {
	AfterCollection []*AfterCollection `json:"afterCollection,omitempty" yaml:"afterCollection,omitempty"`
	Collectors      []*Collect         `json:"collectors,omitempty" yaml:"collectors,omitempty"`
	HostCollectors  []*HostCollect     `json:"hostCollectors,omitempty" yaml:"hostCollectors,omitempty"`
	Analyzers       []*Analyze         `json:"analyzers,omitempty" yaml:"analyzers,omitempty"`
}

// SupportBundleStatus defines the observed state of SupportBundle
type SupportBundleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SupportBundle is the Schema for the SupportBundles API
// +k8s:openapi-gen=true
type SupportBundle struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   SupportBundleSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status SupportBundleStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SupportBundleList contains a list of SupportBundle
type SupportBundleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SupportBundle `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SupportBundle{}, &SupportBundleList{})
}
