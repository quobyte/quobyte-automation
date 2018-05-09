package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type QuobyteService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec QuobyteServiceSpec `json:",spec"`
}

// QuobyteServiceSpec contains spec for quobyte client resource.
type QuobyteServiceSpec struct {
	RegistryService `json:"registry,omitempty"`
	APIService      `json:"api,omitempty"`
	DataService     `json:"data,omitempty"`
	MetadataService `json:"metadata,omitempty"`
}
type Service struct {
	Nodes         []string `json:nodes`
	Image         string   `json:"image"`
	RollingUpdate bool     `json:"rolling_updates_enabled"`
}
type RegistryService struct {
	Service
}

type DataService struct {
	Service
}

type MetadataService struct {
	Service
}

type APIService struct {
	Service
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuobyteServicesList is list of quobyte services
type QuobyteServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []QuobyteService `json:"items"`
}
