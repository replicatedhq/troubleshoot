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

// AnalyzerSpec defines the desired state of Analyzer
type AnalyzerSpec struct {
	Analyzers     []*Analyze     `json:"analyzers,omitempty"`
	HostAnalyzers []*HostAnalyze `json:"hostAnalyzers,omitempty" yaml:"hostAnalyzers,omitempty"`
}

// AnalyzerStatus defines the observed state of Analyzer
type AnalyzerStatus struct {
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Analyzer is the Schema for the analyzers API
// +k8s:openapi-gen=true
type Analyzer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AnalyzerSpec   `json:"spec,omitempty"`
	Status AnalyzerStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// AnalyzerList contains a list of Analyzer
type AnalyzerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Analyzer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Analyzer{}, &AnalyzerList{})
}
