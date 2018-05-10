package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type QuobyteClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec QuobyteClientSpec `json:",spec"`
}

// QuobyteClientSpec contains spec for quobyte client resource.
type QuobyteClientSpec struct {
	Nodes   []string `json:"nodes"`
	Version string   `json:"version"`
}

//QClientContainer quobyte client container image details.
// type QClientContainer struct {
// 	Name  string `json:"- name"`
// 	Image string `json:"image"`
// }

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QuobyteClientList is list of quobyteclients
type QuobyteClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []QuobyteClient `json:"items"`
}
