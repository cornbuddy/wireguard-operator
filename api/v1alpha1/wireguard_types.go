package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// WireguardSpec defines the desired state of Wireguard
type WireguardSpec struct {
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=3
	// +kubebuilder:validation:ExclusiveMaximum=false
	// +kubebuilder:default=1

	// Replicas defines the number of Wireguard instances
	Replicas int32 `json:"replicas,omitempty"`

	// +kubebuilder:default=51820

	// Port defines the port that will be used to init the container with the image
	ContainerPort int32 `json:"containerPort,omitempty"`

	// +kubebuilder:default="192.168.254.253/30"

	// Address space to use
	Address string `json:"network,omitempty"`

	// +kubebuilder:default="localhost"

	// Public address to the peer created
	EndpointAddress string `json:"endpointAddress,omitempty"`

	// FIXME: defaults for the struct provided twice
	// +kubebuilder:default={enabled: true, image: "docker.io/klutchell/unbound:v1.17.1"}

	// Provides configuration of the dns sidecar
	ExternalDNS ExternalDNS `json:"externalDns,omitempty"`

	// +optional

	// Sidecar containers to run
	Sidecars []corev1.Container `json:"sidecars,omitempty"`
}

type ExternalDNS struct {
	// +kubebuilder:default=true

	// Indicates whether to enable external dns
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:default="docker.io/klutchell/unbound:v1.17.1"

	// Image defines the image of the unbound container
	Image string `json:"image,omitempty"`
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
