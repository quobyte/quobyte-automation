package web

import (
	"fmt"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	REGISTRY_LABEL = "quobyte_registry=true"
	DATA_LABEL     = "quobyte_data=true"
	METADATA_LABEL = "quobyte_metadata=true"
	CLIENT_LABEL   = "quobyte_client=true"
)

var (
	quobyteNamespace = "quobyte"
)

// GetPodsWithSelector returns client pods with specified version.
func GetPods(selector, nodeName string) (*v1.PodList, error) {
	podlist, err := K8sAPIClient.CoreV1().Pods(quobyteNamespace).List(metav1.ListOptions{LabelSelector: selector, FieldSelector: fmt.Sprintf("spec.nodeName=%s", nodeName)})
	if err != nil {
		glog.Errorf("Failed to get pods with selector %s due to %s", selector, err)
		return nil, err
	}
	return podlist, nil
}
