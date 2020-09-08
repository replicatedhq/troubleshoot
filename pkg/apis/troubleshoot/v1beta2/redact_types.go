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

// RedactorSpec defines the desired state of Redactor
type RedactorSpec struct {
	Redactors []*Redact `json:"redactors,omitempty"`
}

// RedactorStatus defines the observed state of Redactor
type RedactorStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Redactor is the Schema for the redaction API
// +k8s:openapi-gen=true
type Redactor struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RedactorSpec   `json:"spec,omitempty"`
	Status RedactorStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RedactorList contains a list of Redactor
type RedactorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Redactor `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Redactor{}, &RedactorList{})
}
