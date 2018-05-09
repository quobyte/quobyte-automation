package utils

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func GetVersionFromString(image string) string {
	return image[strings.LastIndex(image, ":")+1:]
}

// GetQuobyteNodes Get Quobyte nodes by the lable. ex; quobyte_registry=true
func GetQuobyteNodes(label string, K8sAPIClient *kubernetes.Clientset) (*v1.NodeList, error) {
	nodes, err := K8sAPIClient.CoreV1().Nodes().List(metav1.ListOptions{LabelSelector: fmt.Sprintf("%s", label)})
	if err != nil {
		glog.Errorf("error listing nodes: %v", err)
		return nodes, err
	}
	return nodes, nil
}
