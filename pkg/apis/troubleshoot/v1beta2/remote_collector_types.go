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

// RemoteCollectorSpec defines the desired state of the RemoteCollector
type RemoteCollectorSpec struct {
	Collectors      []*RemoteCollect   `json:"collectors,omitempty" yaml:"collectors,omitempty"`
	AfterCollection []*AfterCollection `json:"afterCollection,omitempty" yaml:"afterCollection,omitempty"`
	NodeSelector    map[string]string  `json:"nodeSelector,omitempty" yaml:"nodeSelector,omitempty"`
	Uri             string             `json:"uri,omitempty" yaml:"uri,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RemoteCollector is the Schema for the remote collectors API
// +k8s:openapi-gen=true
type RemoteCollector struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   RemoteCollectorSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status CollectorStatus     `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RemoteCollectorList contains a list of RemoteCollectors
type RemoteCollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RemoteCollector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RemoteCollector{}, &RemoteCollectorList{})
}
