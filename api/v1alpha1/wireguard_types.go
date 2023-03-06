package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ExternalDNS struct {
	// Indicates whether to enable external dns
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Enabled bool `json:"enabled,omitempty"`

	// Image defines the image of the unbound container
	// +kubebuilder:default="docker.io/klutchell/unbound:v1.17.1"
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Image string `json:"image,omitempty"`
}

// WireguardSpec defines the desired state of Wireguard
type WireguardSpec struct {
	// Replicas defines the number of Wireguard instances
	// The following markers will use OpenAPI v3 schema to validate the value
	// More info: https://book.kubebuilder.io/reference/markers/crd-validation.html
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3
	// +kubebuilder:validation:ExclusiveMaximum=false
	// +kubebuilder:default=1
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Replicas int32 `json:"replicas,omitempty"`

	// Port defines the port that will be used to init the container with the image
	// +kubebuilder:default=51820
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ContainerPort int32 `json:"containerPort,omitempty"`

	// Address space to use
	// +kubebuilder:default="192.168.254.253/30"
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	Address string `json:"network,omitempty"`

	// Provides configuration of the unbound sidecar
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	ExternalDNS ExternalDNS `json:"externalDns,omitempty"`

	// Public address to the peer created
	// +kubebuilder:default="localhost"
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	EndpointAddress string `json:"endpointAddress,omitempty"`
}

// WireguardStatus defines the observed state of Wireguard
type WireguardStatus struct {
	// Represents the observations of a Wireguard's current state.
	// Wireguard.status.conditions.type are: "Available", "Progressing", and
	// "Degraded"
	// Wireguard.status.conditions.status are one of True, False, Unknown.
	// Wireguard.status.conditions.reason the value should be a CamelCase
	// string and producers of specific condition types may define expected
	// values and meanings for this field, and whether the values are
	// considered a guaranteed API.
	// Wireguard.status.conditions.Message is a human readable message
	// indicating details about the transition.
	// For further information see:
	// https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md#typical-status-properties
	Conditions []metav1.Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Wireguard is the Schema for the wireguards API
type Wireguard struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WireguardSpec   `json:"spec,omitempty"`
	Status WireguardStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// WireguardList contains a list of Wireguard
type WireguardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Wireguard `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Wireguard{}, &WireguardList{})
}
