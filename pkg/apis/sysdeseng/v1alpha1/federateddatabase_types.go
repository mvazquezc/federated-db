package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FederatedDatabaseSpec defines the desired state of FederatedDatabase
// +k8s:openapi-gen=true
type FederatedDatabaseSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	DeploymentSize int32 `json:"size"`
	Replicas       int32 `json:"replicas"`
}

// FederatedDatabaseStatus defines the observed state of FederatedDatabase
// +k8s:openapi-gen=true
type FederatedDatabaseStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book.kubebuilder.io/beyond_basics/generating_crd.html
	Health string `json:"health"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FederatedDatabase is the Schema for the federateddatabases API
// +k8s:openapi-gen=true
type FederatedDatabase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FederatedDatabaseSpec   `json:"spec,omitempty"`
	Status FederatedDatabaseStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FederatedDatabaseList contains a list of FederatedDatabase
type FederatedDatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FederatedDatabase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FederatedDatabase{}, &FederatedDatabaseList{})
}
