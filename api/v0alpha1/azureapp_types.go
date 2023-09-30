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

package v0alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AzureAppSpec defines the desired state of AzureApp
type AzureAppSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Url will be the primary url for your app, used both in Azure App IdentifierURI field and Ingress
	Url string `json:"url,omitempty"`
	// IdentifierURI will be used to set the identifierUri field on Azure app registration
	IdentifierURI string `json:"identifierUri,omitempty"`
	// Identifier will be used on app registration name on Azure and kubernetes resources
	Identifier string `json:"identifier,omitempty"`
	// ServingPort will be used to set the port configuration on your service - the node port will still be random
	ServingPort int32 `json:"servingPort,omitempty"`
	// ContainerImage will set the app's image
	ContainerImage string `json:"containerImage,omitempty"`
	// AppRoles will be used to set app registration roles on Azure
	AppRoles []string `json:"appRoles,omitempty"`
	// EnvVars will set the app's environment variables
	EnvVars map[string]string `json:"envVars,omitempty"`
	// EnableDatabase will set if an Azure Sql Database should be created
	EnableDatabase bool `json:"enableDatabase,omitempty"`
}

// AzureAppStatus defines the observed state of AzureApp
type AzureAppStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Deployment        string `json:"deployment,omitempty"`
	ProvisioningState string `json:"provisioningState,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:JSONPath=".status.deployment",name="Deployment",type="string"
//+kubebuilder:printcolumn:JSONPath=".status.provisioningState",name="ProvisioningState",type="string"
//+kubebuilder:printcolumn:JSONPath=".metadata.creationTimestamp",name="Age",type="date"

// AzureApp is the Schema for the azureapps API
type AzureApp struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AzureAppSpec   `json:"spec,omitempty"`
	Status AzureAppStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AzureAppList contains a list of AzureApp
type AzureAppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AzureApp `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AzureApp{}, &AzureAppList{})
}
