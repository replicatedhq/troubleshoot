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

type ResultRequest struct {
	URI       string `json:"uri" yaml:"uri"`
	Method    string `json:"method" yaml:"method"`
	RedactURI string `json:"redactUri" yaml:"redactUri"` // the URI to POST redaction reports to
}

type AfterCollection struct {
	UploadResultsTo    *ResultRequest          `json:"uploadResultsTo,omitempty" yaml:"uploadResultsTo,omitempty"`
	Callback           *ResultRequest          `json:"callback,omitempty" yaml:"callback,omitempty"`
	UploadToReplicated *UploadToReplicatedSpec `json:"uploadToReplicated,omitempty" yaml:"uploadToReplicated,omitempty"`
}

// UploadToReplicatedSpec configures uploading support bundles to the Replicated
// vendor portal via presigned S3 URLs. Credentials are auto-discovered from the
// live cluster by reading the Replicated SDK Kubernetes Secret.
//
// SDK secret discovery:
//   - Found by the label helm.sh/chart with prefix "replicated-"
//   - Name follows the convention {APP_NAME}-sdk (e.g., "firstresponse-sdk")
//   - License ID is read from the "config.yaml" key in the secret data
//   - Falls back to "integration-license-id" key if config.yaml is absent
//
// Upload flow:
//  1. Get a presigned S3 URL from POST /v3/supportbundle/upload-url
//  2. PUT the bundle archive directly to S3
//  3. Notify the API via POST /v3/supportbundle/{bundleID}/uploaded
//
// RBAC: The service account needs "get" and "list" on secrets in the target
// namespace, or cluster-wide for cross-namespace discovery.
// See examples/support-bundle/upload-to-replicated.yaml for a complete example.
type UploadToReplicatedSpec struct {
	// SecretName overrides the auto-discovered SDK secret name.
	// By default, the secret is found by its helm.sh/chart label.
	// +optional
	SecretName string `json:"secretName,omitempty" yaml:"secretName,omitempty"`
	// SecretNamespace overrides the namespace to look for the SDK secret.
	// Defaults to the namespace troubleshoot is running in.
	// +optional
	SecretNamespace string `json:"secretNamespace,omitempty" yaml:"secretNamespace,omitempty"`
	// Endpoint overrides the Replicated API endpoint. Must use HTTPS.
	// Defaults to the value from the SDK secret, or https://replicated.app.
	// +optional
	Endpoint string `json:"endpoint,omitempty" yaml:"endpoint,omitempty"`
}

// CollectorSpec defines the desired state of Collector
type CollectorSpec struct {
	Collectors      []*Collect         `json:"collectors,omitempty" yaml:"collectors,omitempty"`
	HostCollectors  []*HostCollect     `json:"hostCollectors,omitempty" yaml:"hostCollectors,omitempty"`
	AfterCollection []*AfterCollection `json:"afterCollection,omitempty" yaml:"afterCollection,omitempty"`
	Uri             string             `json:"uri,omitempty" yaml:"uri,omitempty"`
}

// CollectorStatus defines the observed state of Collector
type CollectorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Collector is the Schema for the collectors API
// +k8s:openapi-gen=true
type Collector struct {
	metav1.TypeMeta   `json:",inline" yaml:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Spec   CollectorSpec   `json:"spec,omitempty" yaml:"spec,omitempty"`
	Status CollectorStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CollectorList contains a list of Collector
type CollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Collector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Collector{}, &CollectorList{})
}
