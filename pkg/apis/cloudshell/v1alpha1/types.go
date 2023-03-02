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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ExposeMode describes how to access ttyd service, either ClusterIP, NodePort, Ingress or VirtualService.
// +enum
type ExposureMode string

const (
	ExposureServiceClusterIP ExposureMode = "ClusterIP"
	ExposureServiceNodePort  ExposureMode = "NodePort"
	ExposureIngress          ExposureMode = "Ingress"
	ExposureVirtualService   ExposureMode = "VirtualService"

	PhaseCreatedJob   = "CreatedJob"
	PhaseCreatedRoute = "CreatedRouteRule"
	PhaseReady        = "Ready"
	PhaseCompleted    = "Complete"
	PhaseFailed       = "Failed"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required. Any new fields you add must have json tags for the fields to be serialized.

// CloudShellSpec defines the desired state of CloudShell
type CloudShellSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// SecretRef represents the secret contains mandatory credentials to access the target cluster.
	// The secret should hold credentials as follows:
	// - secret.data.token
	// - secret.data.caBundle
	// The field is alpha phase, paleas open the featuregate AllowSecretStoreKubeconfig to use it.
	// +optional
	SecretRef *LocalSecretReference `json:"secretRef,omitempty"`

	// +optional
	RunAsUser string `json:"runAsUser,omitempty"`

	// Image defines the image to cloudshell, we can customize an image.
	// Note: the image must be built on top of the officially available base image.
	// Please see: https://github.com/cloudtty/cloudtty#how-to-build-custom-cloudshell-image
	// +optional
	Image string `json:"image,omitempty"`

	// accept only one client and exit on disconnection
	// +optional
	Once bool `json:"once,omitempty"`

	// +optional
	CommandAction string `json:"commandAction,omitempty"`

	// +optional
	// TODO: repalce type `int32` of ttl with `time`.
	Ttl int32 `json:"ttl,omitempty"`

	// Cleanup specified whether to delete cloudshell resources when corresponding job status is completed.
	// +optional
	Cleanup bool `json:"cleanup,omitempty"`

	// +kubebuilder:validation:Enum=ClusterIP;NodePort;Ingress;VirtualService
	// + optional
	ExposeMode ExposureMode `json:"exposureMode,omitempty"`

	// Specifies a port number range 30000-32767 when using nodeport mode,
	// if not specified, kubernetes default random rule is used.
	// NodePort int32 `json:"NodePort,omitempty"`
	// IngressConfig specifies necessary parameters to create ingress.
	// +optional
	IngressConfig *IngressConfig `json:"ingressConfig,omitempty"`

	// VirtualServiceConfig specifies some of the parameters necessary to create the virtaulService.
	// +optional
	VirtualServiceConfig *VirtualServiceConfig `json:"virtualServiceConfig,omitempty"`

	// PathPrefix specified a path prefix to access url, if not, the default path is used.
	// +optional
	PathPrefix string `json:"pathPrefix,omitempty"`

	// PathSuffix specified a path suffix to access url, if not, the default path is used.
	// +optional
	PathSuffix string `json:"pathSuffix,omitempty"`

	// UrlArg allow client to send command line arguments in URL (eg: http://localhost:7681?arg=foo&arg=bar)
	// +optional
	UrlArg bool `json:"urlArg,omitempty"`

	// List of environment variables to set in the container.
	// Cannot be updated.
	// +optional
	// +patchMergeKey=name
	// +patchStrategy=merge
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// VirtualServiceConfig specifies some of the parameters necessary to create the virtaulService.
type VirtualServiceConfig struct {
	// VirtualServiceName specifies a name to virtualService, if it's
	// empty, default "cloudshell-VirtualService"
	VirtualServiceName string `json:"virtualServiceName,omitempty"`

	// Namespace specifies a namespace that the virtualService will be
	// created in it. if it's empty, default the cloudshell namespace.
	Namespace string `json:"namespace,omitempty"`

	// The value "." is reserved and defines an export to the same namespace that
	// the virtual service is declared in. Similarly the value "*" is reserved and
	// defines an export to all namespaces.
	ExportTo string `json:"export_to,omitempty"`

	// Gateway must be specified and the gateway already exists in the cluster.
	Gateway string `json:"gateway,omitempty"`
}

// IngressConfig specifies some of the parameters necessary to create the ingress.
type IngressConfig struct {
	// IngressName specifies a name to ingress, if it's empty, default "cloudshell-ingress".
	IngressName string `json:"ingressName,omitempty"`

	// Namespace specifies a namespace that the virtualService will be
	// created in it. if it's empty, default the cloudshell namespace.
	Namespace string `json:"namespace,omitempty"`

	// IngressClassName specifies a ingress controller to ingress,
	// it must be fill when the cluster have multiple ingress controller service.
	IngressClassName string `json:"ingressClassName,omitempty"`
}

// LocalSecretReference is a reference to a secret within the enclosing
// namespace.
type LocalSecretReference struct {
	// Name is the name of resource being referenced.
	Name string `json:"name,omitempty"`
}

// CloudShellStatus defines the observed state of CloudShell
type CloudShellStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// +optional
	Phase string `json:"phase"`

	// +optional
	AccessURL string `json:"accessUrl"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name=User,type="string",JSONPath=".spec.runAsUser",description="User"
//+kubebuilder:printcolumn:name=Command,type="string",JSONPath=".spec.commandAction",description="Command"
//+kubebuilder:printcolumn:name=Type,type="string",JSONPath=".spec.exposureMode",description="Expose mode"
//+kubebuilder:printcolumn:name=URL,type="string",JSONPath=".status.accessUrl",description="Access Url"
//+kubebuilder:printcolumn:name=Phase,type="string",JSONPath=".status.phase",description="Phase"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// CloudShell is the Schema for the cloudshells API
type CloudShell struct {
	metav1.TypeMeta `json:",inline"`

	// Standard object's metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired behavior of the cloudshell.
	// +optional
	Spec CloudShellSpec `json:"spec,omitempty"`

	// Most recently observed status of the cloudshell.
	// +optional
	Status CloudShellStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudShellList contains a list of CloudShell
type CloudShellList struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []CloudShell `json:"items"`
}
