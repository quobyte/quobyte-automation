package resourcehandler

import (
	"encoding/json"

	v1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/golang/glog"
	types "k8s.io/apimachinery/pkg/types"
	"operator/pkg/utils"
)

// GetPods Returns all the pods in cluster
func GetPods(client *kubernetes.Clientset) (*v1.PodList, error) {
	podList, err := client.CoreV1().Pods(quobyteNameSpace).List(metav1.ListOptions{})
	return podList, err
}

// LabelNodes updates labels on nodes.
// op add,remove
func LabelNodes(labelNodes []string, op, label string) {

	for _, nodeName := range labelNodes {
		if nodeName == "" {
			continue
		}
		node, err := KubernetesClient.CoreV1().Nodes().Get(nodeName, metav1.GetOptions{})
		if err != nil {
			glog.Errorf("Failed to get the node %s due to %v", nodeName, err)
		} else {
			oldData, _ := json.Marshal(node)
			labels := node.GetLabels()
			if labels == nil {
				labels = make(map[string]string)
			}
			switch op {
			case utils.OperationAdd:
				labels[label] = "true"
			case utils.OperationRemove:
				delete(labels, label)
			}
			node.SetLabels(labels)
			newJSON, _ := json.Marshal(node)
			patchbytes, _ := strategicpatch.CreateTwoWayMergePatch(oldData, newJSON, v1.Node{})
			if len(patchbytes) > 2 {
				_, err = KubernetesClient.CoreV1().Nodes().Patch(node.Name, types.StrategicMergePatchType, patchbytes)
				if err != nil {
					glog.Errorf("Failed labeling node %s due to %v", node.Name, err)
				}
			}
		}
	}
}

// AddUpgradeTaint added while quobyte client version update.
// Taint serves better our case than drain (as drain cannot delete pods deployed with daemonset, deployments)
// With taint daemonset and deployment cannot see the label on node and that allows us to delete pod without being recreated before client update.
func AddUpgradeTaint(name string) {
	if name == "" {
		return
	}

	node, err := KubernetesClient.CoreV1().Nodes().Get(name, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("Failed to get the node %s due to %v", name, err)
	}

	taints := node.Spec.Taints

	for _, taint := range taints {
		if taint.Key == "quobyte-upgrade" {
			return
		}
	}

	oldJSON, _ := json.Marshal(node)

	taint := v1.Taint{
		Key:    "quobyte-upgrade",
		Value:  "true",
		Effect: "NoSchedule",
		// TimeAdded: time.Now(),
	}

	node.Spec.Taints = append(taints, taint)

	newJSON, _ := json.Marshal(node)
	patchbytes, _ := strategicpatch.CreateTwoWayMergePatch(oldJSON, newJSON, v1.Node{})
	if len(patchbytes) > 2 {
		_, err = KubernetesClient.CoreV1().Nodes().Patch(node.Name, types.StrategicMergePatchType, patchbytes)
		if err != nil {
			glog.Errorf("Failed labeling node %s due to %v", node.Name, err)
		}
	}
}

func RemoveUpgradeTaint(name string) {
	if name == "" {
		return
	}

	node, err := KubernetesClient.CoreV1().Nodes().Get(name, metav1.GetOptions{})
	if err != nil {
		glog.Errorf("Failed to get the node %s due to %v", name, err)
	}

	taints := node.Spec.Taints

	for i, taint := range taints {
		if taint.Key == "quobyte-upgrade" {
			index := i
			taints = append(taints[:index], taints[index+1:]...)
			break
		}
	}
	oldJSON, _ := json.Marshal(node)
	node.Spec.Taints = taints
	newJSON, _ := json.Marshal(node)
	patchbytes, _ := strategicpatch.CreateTwoWayMergePatch(oldJSON, newJSON, v1.Node{})
	if len(patchbytes) > 2 {
		_, err = KubernetesClient.CoreV1().Nodes().Patch(node.Name, types.StrategicMergePatchType, patchbytes)
		if err != nil {
			glog.Errorf("Failed labeling node %s due to %v", node.Name, err)
		}
	}
}
