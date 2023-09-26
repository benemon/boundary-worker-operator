/*
Copyright 2023.

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// BoundaryPKIWorkerSpec defines the desired state of BoundaryPKIWorker
type BoundaryPKIWorkerSpec struct {

	// The Registration block containing configurations required to register this Boundary Worker with its cluster
	Registration BoundaryPKIWorkerRegistrationSpec `json:"registration"`

	// The Resources block containing runtime resource requirements for the Boundary Worker
	Resources BoundaryPKIWorkerResourcesSpec `json:"resources,omitempty"`

	//The tags block containg a map of extra, custom tags to associate with the Boundary Worker. Supports comma-seperated values.
	Tags map[string]string `json:"tags,omitempty"`
}

type BoundaryPKIWorkerRuntimeSpec struct {
	// The CPU block containing the Boundary Worker CPU requirements
	CPU string `json:"cpu,omitempty"`
	// The Memory block containing the Boundary Worker CPU requirements
	Memory string `json:"memory,omitempty"`
}

type BoundaryPKIWorkerResourcesSpec struct {
	// The Storage block containing the Boundary Worker storage configuration
	Storage BoundaryPKIWorkerStorageSpec `json:"storage,omitempty"`
	// The Requests block containing the Boundary Worker CPU and Memory requests
	Requests BoundaryPKIWorkerRuntimeSpec `json:"requests,omitempty"`
	// The Limits block containing the Boundary Worker CPU and Memory limits
	Limits BoundaryPKIWorkerRuntimeSpec `json:"limits,omitempty"`
}

type BoundaryPKIWorkerStorageSpec struct {

	// StorageClass to use. Will use default storage class if omitted
	StorageClassName string `json:"storageClassName,omitempty"`
}

type BoundaryPKIWorkerRegistrationSpec struct {
	// HCPBoundaryClusterID accepts a Boundary cluster id and will be used by a worker when initially connecting to HCP Boundary. Your cluster id is the UUID in the controller url.
	HCPBoundaryClusterID string `json:"hcpBoundaryClusterID"`
	// ControllerGeneratedActivationToken is the token retrieved by the administrator from Boundary. Once the worker starts, it reads this token and uses it to authorize to the cluster. Note that this token is one-time-use; it is safe to keep it here even after the worker has successfully authorized and authenticated, as it will be unusable at that point.
	ControllerGeneratedActivationToken string `json:"controllerGeneratedActivationToken,omitempty"`
}

// BoundaryPKIWorkerStatus defines the observed state of BoundaryPKIWorker
type BoundaryPKIWorkerStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// BoundaryPKIWorker is the Schema for the boundarypkiworkers API
type BoundaryPKIWorker struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BoundaryPKIWorkerSpec   `json:"spec,omitempty"`
	Status BoundaryPKIWorkerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// BoundaryPKIWorkerList contains a list of BoundaryPKIWorker
type BoundaryPKIWorkerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BoundaryPKIWorker `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BoundaryPKIWorker{}, &BoundaryPKIWorkerList{})
}
