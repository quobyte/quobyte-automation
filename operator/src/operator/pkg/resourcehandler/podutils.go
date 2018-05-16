package resourcehandler

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"operator/pkg/utils"
	"strings"
	"time"

	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	podCreateMaxRetries int = 5
)

// ClientUpdateOnHold Client pod that is not updated by operator.
// Operator may not update some pods due to early exist of updating on first update failure or
// node has some application pods that are accessing the QuobyteVolume either through PVC/PV or Volume reference.
type ClientUpdateOnHold struct {
	Node          string // node on which client update is held by pods
	Pod           string
	ExpectedImage string
	CurrentImage  string
	BlockingPods  []string // pods using QuobyteVolume
}

// ServiceNotUpdated Service pod (registry,metadata,data) that is not updated by operator.
// Operator quits updating any pod of the same service on first failure i.e; if first registry pod update failed then no further registries will be updated.
type ServiceNotUpdated struct {
	Node          string // node on which client update is held by pods
	Pod           string
	ExpectedImage string
	CurrentImage  string
}

// GetPodsWithSelector gives pods with specified selector. The selector must be valid label selector supported by API.
func GetPodsWithSelector(selector string) (podlist *v1.PodList) {
	podlist, err := KubernetesClient.CoreV1().Pods(quobyteNameSpace).List(metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		glog.Errorf("Failed to get pods with selector %s due to %s", selector, err)
		return nil
	}
	return
}

// DeletePods deletes pods by given selector.
func DeletePods(selector string) {
	list := GetPodsWithSelector(selector)
	if list != nil {
		for _, pod := range list.Items {
			err := KubernetesClient.CoreV1().Pods(quobyteNameSpace).Delete(pod.Name, &metav1.DeleteOptions{})
			if err != nil {
				glog.Errorf("Failed to delete pod %s due to %s", pod.Name, err)
			}
		}
	}
}

// ControlledPodUpdate Deletes given list of pods. This triggers redeployment of updated pods.
// Currently, this method checks pod recreation on node 5 times with each try dealyed by 60 secs.
// If Pod not running within 5 tries past the Pending pod status, this reports Failure and exits update process.
// TODO: Refactor this method.
func ControlledPodUpdate(role, image string, update bool) {
	podList := GetPodsWithSelector(fmt.Sprintf("role=%s", role))
	ClientStatus := make([]*ClientUpdateOnHold, 0, len(podList.Items))
	serviceStatus := make([]*ServiceNotUpdated, 0, len(podList.Items))
	failureMessage := "" // first update pod sets the message, stopping all other pods of the same service from updating.
	for _, pod := range podList.Items {
		if pod.Spec.Containers[0].Image == image {
			glog.Infof("%s pod already has the requsted image %s", pod.Name, image)
			continue
		}
		var client bool
		if role == "client" {
			client = true
			clientOnHoldPods := checkQuobyteVolumeMounts(pod, image,update)
			if clientOnHoldPods != nil {
				ClientStatus = append(ClientStatus, clientOnHoldPods)
				fmt.Printf("pods with QB volumes %v\n", clientOnHoldPods)
				continue
			} else if len(failureMessage) > 0 { // if alread a client failed then don't proceed to upgrade
				ClientStatus = append(ClientStatus, &ClientUpdateOnHold{pod.Spec.NodeName, pod.Name, image, pod.Spec.Containers[0].Image, nil})
				continue
			}
		} else {
			if len(failureMessage) > 0 || !update {
				serviceStatus = append(serviceStatus, &ServiceNotUpdated{pod.Spec.NodeName, pod.Name, image, pod.Spec.Containers[0].Image})
			}
		}
		if update {
			if len(failureMessage) == 0 { // No previous failure, so continue update.
				fmt.Printf("deleting pod: for the role %s %s %d %t\n", role, pod.Name, len(failureMessage), update)
				err = KubernetesClient.CoreV1().Pods(quobyteNameSpace).Delete(pod.Name, &metav1.DeleteOptions{})
				if err != nil {
					if client {
						RemoveUpgradeTaint(pod.Spec.NodeName)
					}
					glog.Errorf("Exiting %s update. Failed to delete pod %s due to %v", role, pod.Name, err)
				}
				podRunning := false
				retryCount := 0
				for !podRunning && retryCount < podCreateMaxRetries {
					podList, err := KubernetesClient.CoreV1().Pods(quobyteNameSpace).List(metav1.ListOptions{LabelSelector: fmt.Sprintf("role=%s", role), FieldSelector: fmt.Sprintf("spec.nodeName=%s", pod.Spec.NodeName)})
					if err != nil {
						glog.Errorf("Failed to list updated %s pod on node %s due to %v", role, pod.Spec.NodeName, err)
						failureMessage = fmt.Sprintf("Failed to list updated %s pod on node %s due to %v", role, pod.Spec.NodeName, err)
						if client {
							RemoveUpgradeTaint(pod.Spec.NodeName)
						}
						continue
					}
					if len(podList.Items) > 0 {
						podRunning, err = isPodRunning(podList.Items[0])
						if !podRunning {
							if err == nil { // Count retry only if pod is past pod Pending status (completed containers download)
								retryCount++
							}
							glog.Infof("Updated Pod is not running on node %s\nretry: %d of configured max retries: %d", pod.Spec.NodeName, retryCount, podCreateMaxRetries)
							time.Sleep(time.Minute)
						} else {
							glog.Infof("%s pod is running. Continuing next pod update", podList.Items[0].Name)
							time.Sleep(time.Minute)
						}
					}
				}
				if !podRunning {
					glog.Errorf("Exiting update: Failed to create updated %s pod within %d retries", role, retryCount)
					failureMessage = fmt.Sprintf("Failed to create updated %s pod within %d retries, no further client pods are updated by operator", role, retryCount)
					if client {
						RemoveUpgradeTaint(pod.Spec.NodeName)
						glog.Error(failureMessage)
					}
				}
				if client {
					RemoveUpgradeTaint(pod.Spec.NodeName)
				}
			}
		}
	}
	if !update {
		if role == "client" {
			outStanding, err := json.Marshal(ClientStatus)
			if err != nil {
				glog.Errorf("Failed converting outstanding clients to JSON: %v", err)
			}
			// fmt.Printf("current status of the client updates: %v ", string(outStanding))
			err = ioutil.WriteFile(utils.CLIENT_STATUS_FILE, outStanding, 0644)
			if err != nil {
				glog.Errorf("Failed writing status to file %v", err)
			}
		} else {
			saveServiceStatus(serviceStatus, role)
		}
	}
}

func saveServiceStatus(status []*ServiceNotUpdated, role string) {
	outStanding, err := json.Marshal(status)
	if err != nil {
		glog.Errorf("Failed converting outstanding clients to JSON: %v", err)
	}
	// fmt.Printf("current status of the %s updates: %v ", role, string(outStanding))
	err = ioutil.WriteFile("/public/"+fmt.Sprintf("%s-status.json", role), outStanding, 0644)
	if err != nil {
		glog.Errorf("Failed writing status to file %v", err)
	}
}

func checkQuobyteVolumeMounts(pod v1.Pod, image string,update bool) *ClientUpdateOnHold {
    if update{
	// Don't let any pod schedule on this node temporarily.
	AddUpgradeTaint(pod.Spec.NodeName)
	}
	lblSelector := "role  notin (registry,client,data,metadata,data,qmgmt-pod,webconsole)"
	pods, err := KubernetesClient.CoreV1().Pods("").List(metav1.ListOptions{LabelSelector: lblSelector, FieldSelector: fmt.Sprintf("spec.nodeName=%s", pod.Spec.NodeName)})
	if err != nil {
		if update {
			RemoveUpgradeTaint(pod.Spec.NodeName)
		}
		message := fmt.Sprintf("Skipping client update. Cannot check the pods accessing Quobyte volume. Failed to get the pods on %s due to %v", pod.Spec.NodeName, err)
		glog.Error(message)
		return &ClientUpdateOnHold{pod.Spec.NodeName, pod.Name, image, pod.Spec.Containers[0].Image, nil}
	}
	QuobyteVolumePods := make([]string, 0, len(pods.Items))
	for _, pod := range pods.Items {
		for _, volume := range pod.Spec.Volumes {
			if volume.VolumeSource.Quobyte != nil {
				// fmt.Printf("pod: %s has Quobyte volume %v\n", pod.Name, volume.VolumeSource.Quobyte)
				QuobyteVolumePods = append(QuobyteVolumePods, pod.Name)
				glog.Infof("%s pod is using Quobyte volume. Client update needs to be done by administrator.", pod.Name)
				glog.Infof("To update client on node %s, remove the pod(s) manually and remove the client (removing client triggers updated client scheduling on node)", pod.Spec.NodeName)
			} else if volume.VolumeSource.PersistentVolumeClaim != nil {
				claim := volume.VolumeSource.PersistentVolumeClaim.ClaimName
				pvc, err := KubernetesClient.CoreV1().PersistentVolumeClaims(pod.Namespace).Get(claim, metav1.GetOptions{})
				if err != nil {
					glog.Errorf("Unable to get the PVC for pod %s for the claim %s due to %v", pod.Name, claim, err)
					continue
				}
				storageClass, err := KubernetesClient.StorageV1().StorageClasses().Get(*pvc.Spec.StorageClassName, metav1.GetOptions{})
				if err != nil {
					glog.Errorf("Unable to get the storage class %s for the PVC %s", storageClass, pvc.Name)
					continue
				}
				if strings.Contains(storageClass.Provisioner, "quobyte") {
					QuobyteVolumePods = append(QuobyteVolumePods, pod.Name)
				}
			}
		}
	}
	if update{
		RemoveUpgradeTaint(pod.Spec.NodeName)
	}
	if len(QuobyteVolumePods) > 0 {
		return &ClientUpdateOnHold{pod.Spec.NodeName, pod.Name, image, pod.Spec.Containers[0].Image, QuobyteVolumePods}
	}
	return nil
}

func isPodRunning(pod v1.Pod) (bool, error) {
	switch pod.Status.Phase {
	case v1.PodFailed, v1.PodSucceeded:
		return false, nil
	case v1.PodRunning:
		for _, cond := range pod.Status.Conditions {
			if cond.Type != v1.PodReady {
				continue
			}
			return cond.Status == v1.ConditionTrue, nil
		}
		return false, nil
	case v1.PodPending:
		return false, fmt.Errorf("Pending")
	}
	return false, nil
}
