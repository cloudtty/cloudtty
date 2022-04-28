/*
Copyright 2022.

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

// CloudShellSpec defines the desired state of CloudShell
type CloudShellSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Configmap of the target kube-config, will replace by SA
	ConfigmapName string `json:"configmapName,omitempty"`
	RunAsUser     string `json:"runAsUser,omitempty"`
	// accept only one client and exit on disconnection
	Once          bool   `json:"once,omitempty"`
	CommandAction string `json:"commandAction,omitempty"`
	Ttl           int32  `json:"ttl,omitempty"`
}

// CloudShellStatus defines the observed state of CloudShell
type CloudShellStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Phase     string `json:"phase"`
	AccessURL string `json:"accessUrl"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name=User,type="string",JSONPath=".spec.runAsUser",description="User"
//+kubebuilder:printcolumn:name=Command,type="string",JSONPath=".spec.commandAction",description="Command"
//+kubebuilder:printcolumn:name=URL,type="string",JSONPath=".status.accessUrl",description="Access Url"
//+kubebuilder:printcolumn:name=Phase,type="string",JSONPath=".status.phase",description="Phase"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// CloudShell is the Schema for the cloudshells API
type CloudShell struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudShellSpec   `json:"spec,omitempty"`
	Status CloudShellStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudShellList contains a list of CloudShell
type CloudShellList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudShell `json:"items"`
}
