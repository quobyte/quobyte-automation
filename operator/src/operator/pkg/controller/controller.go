package controller

import (
	"encoding/json"
	"fmt"
	quobytev1 "operator/pkg/kubernetes-actors/clientset/versioned"
	quobyte_crd_informer_factory "operator/pkg/kubernetes-actors/informers/externalversions"
	"operator/pkg/resourcehandler"
	"operator/pkg/utils"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"time"

	glog "github.com/golang/glog"
	"github.com/mattbaird/jsonpatch"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"operator/pkg/api/quobyte.com/v1"
)

type controller struct {
	clientinformer   cache.SharedIndexInformer
	servicesInformer cache.SharedIndexInformer
}

func (controller *controller) Run(stopCh chan struct{}) {
	defer utilruntime.HandleCrash()
	go controller.clientinformer.Run(stopCh)
	go controller.servicesInformer.Run(stopCh)
}

func newQuobyteClientController(quobyteclient *quobytev1.Clientset) (*controller, error) {
	informerFactory := quobyte_crd_informer_factory.NewSharedInformerFactory(quobyteclient, 5*time.Minute)
	quobyteclientInformer := informerFactory.Quobyte().V1().QuobyteClients().Informer()
	quobyteservicesInformer := informerFactory.Quobyte().V1().QuobyteServices().Informer()

	quobyteclientInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			glog.Info("Client CR create event triggered")
			handleClientCRAdd(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			glog.Info("Client CR update event triggered")
			handleClientCRUpdate(old, new)
		},
		DeleteFunc: func(obj interface{}) {
			glog.Info("Client CR delete event triggered")
			handleClientCRDelete(obj)
		},
	})

	quobyteservicesInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			glog.Info("Service CR create event triggered")
			handleServicesAdd(obj)
		},
		UpdateFunc: func(old, new interface{}) {
			glog.Info("Service CR update event triggered")
			handleServicesCRUpdate(old, new)
		},
		DeleteFunc: func(obj interface{}) {
			glog.Info("Service CR delete event triggered")
			handleServicesCRDelete(obj)
		},
	})

	controllerDef := &controller{
		clientinformer:   quobyteclientInformer,
		servicesInformer: quobyteservicesInformer,
	}
	return controllerDef, nil
}

func handleClientCRAdd(obj interface{}) {
	syncQuobyteVersion("client", obj.(*v1.QuobyteClient).Spec.Image)
	resourcehandler.LabelNodes(obj.(*v1.QuobyteClient).Spec.Nodes, "add", "quobyte_client")
	removeNonConfirmingCRNodes(obj.(*v1.QuobyteClient).Spec.Nodes, "quobyte_client")
	// queryClientPodUpToDateness()
}

// Removes any service that is not defined in the Quobyte config definition
func removeNonConfirmingCRNodes(crNodes []string, labelVal string) {
	nodes, err := utils.GetQuobyteNodes(fmt.Sprintf("%s=true", labelVal), resourcehandler.KubernetesClient)
	if err != nil {
		glog.Errorf("Unable to get current nodes for the label %s due to %v", labelVal, err)
		return
	}
	crNode := make(map[string]bool, len(crNodes))
	for _, node := range crNodes {
		crNode[node] = true
	}
	for _, node := range nodes.Items {
		_, ok := crNode[node.Name]
		if !ok {
			glog.Infof("Removing node %s from %s services as the node is not part of the Quobyte Configuration. To deploy, update the required quobyte definition (clients/services) with node.", node.Name, strings.Title(strings.Replace(labelVal, "_", " ", -1)))
			resourcehandler.LabelNodes([]string{node.Name}, "remove", labelVal)
		}
	}
}

func handleClientCRDelete(definition interface{}) {
	resourcehandler.LabelNodes(definition.(*v1.QuobyteClient).Spec.Nodes, "remove", "quobyte_client")
	resourcehandler.DeletePods("role=client")
	glog.Info("Removed Quobyte client definition (Pod termination signalled)")
	removeNonConfirmingCRNodes(definition.(*v1.QuobyteClient).Spec.Nodes, "quobyte_client")
}

func handleClientCRUpdate(old, cur interface{}) {
	oldCr := old.(*v1.QuobyteClient)
	curCr := cur.(*v1.QuobyteClient)

	glog.Info("QuobyteClient CR updated: Updating Quobyte to the updated template.")

	if !reflect.DeepEqual(oldCr.Spec.Nodes, curCr.Spec.Nodes) {
		handleNodeChanges(oldCr, curCr, oldCr.Spec.Nodes, curCr.Spec.Nodes, "quobyte_client")
	}

	if oldCr.Spec.Image != curCr.Spec.Image {
		glog.Infof("Client Image changed from %s to %s", oldCr.Spec.Image, curCr.Spec.Image)
		err := resourcehandler.UpdateDaemonSet("client", curCr.Spec.Image) // TODO: get dynamic name

		if err != nil {
			glog.Errorf("Failed to update client daemonset with updated image %s\n", curCr.Spec.Image)
		}
		if curCr.Spec.RollingUpdate {
			// podSelector := fmt.Sprintf("version=%s,role=client", resourcehandler.GetVersionFromString(oldCr.Spec.Image))
			resourcehandler.ControlledPodUpdate("client", curCr.Spec.Image, true)
		} else {
			printManualUpdateMessage("client")
		}
	}
	removeNonConfirmingCRNodes(curCr.Spec.Nodes, "quobyte_client")
	// queryClientPodUpToDateness()
}

func handleNodeChanges(oldCr, curCr interface{}, oldNodes, curNodes []string, label string) {
	oldCrData, err := json.Marshal(oldCr)
	if err != nil {
		glog.Errorf("Failed Json conversion of old client CR")
	}
	curCrData, err := json.Marshal(curCr)

	if err != nil {
		glog.Errorf("Failed Json conversion of updated client CR")
	}

	patch, err := jsonpatch.CreatePatch([]byte(oldCrData), []byte(curCrData))
	if err != nil {
		glog.Errorf("Error creating JSON patch:%s", err)
		return
	}

	nodesPath := getNodesPath(label)
	removedNodes := make([]string, 0, len(oldNodes)) // at most, only nodes in old definition can be removed.
	addedNodes := make([]string, 0, len(curNodes))   // at most, only nodes in current definition must have been added.
	for _, operation := range patch {
		if strings.Contains(strings.ToLower(operation.Path), nodesPath) {
			if operation.Operation == "remove" {
				//get the removed node index, so that we don't remove/modify the unexpected node.
				index, _ := strconv.Atoi(operation.Path[strings.Index(strings.ToLower(operation.Path), nodesPath)+len(nodesPath) : len(operation.Path)])
				removedNode := oldNodes[index]
				removedNodes = append(removedNodes, removedNode)
			} else if operation.Operation == "add" {
				addedNodes = append(addedNodes, operation.Value.(string))
			} else if operation.Operation == "replace" {
				// remove old node and add new node
				index, _ := strconv.Atoi(operation.Path[strings.Index(strings.ToLower(operation.Path), nodesPath)+len(nodesPath) : len(operation.Path)])
				removedNode := oldNodes[index]
				removedNodes = append(removedNodes, removedNode)
				addedNodes = append(addedNodes, operation.Value.(string))
			}
		}
	}
	if len(removedNodes) > 0 {
		glog.Infof("Removed nodes:%s", removedNodes)
		resourcehandler.LabelNodes(removedNodes, "remove", label)
	}
	if len(addedNodes) > 0 {
		glog.Infof("Added nodes:%s", addedNodes)
		resourcehandler.LabelNodes(addedNodes, "add", label)
	}
}

func getNodesPath(label string) string {
	switch label {
	case "quobyte_client":
		return "/spec/nodes/"
	case "quobyte_registry":
		return "/spec/registry/nodes/"
	case "quobyte_data":
		return "/spec/data/nodes/"
	case "quobyte_metadata":
		return "/spec/metadata/nodes/"
	}
	return "none/node"
}

// Start starts quobyte crd monitoring
func Start(quobyteclient *quobytev1.Clientset) {
	controller, err := newQuobyteClientController(quobyteclient)
	if err != nil {
		glog.Errorf("Terminating operator due to:\n %v ", err)
	}
	stopCh := make(chan struct{})
	defer close(stopCh)

	go controller.Run(stopCh)

	termSig := make(chan os.Signal, 1)
	signal.Notify(termSig, syscall.SIGTERM)
	signal.Notify(termSig, syscall.SIGINT)
	<-termSig
}

func keepServiceNodesInSync(regstryNodes, dataNodes, metaNodes []string) {
	removeNonConfirmingCRNodes(regstryNodes, "quobyte_registry")
	removeNonConfirmingCRNodes(dataNodes, "quobyte_data")
	removeNonConfirmingCRNodes(metaNodes, "quobyte_metadata")
}

func handleServicesAdd(obj interface{}) {
	registry := obj.(*v1.QuobyteService).Spec.RegistryService
	data := obj.(*v1.QuobyteService).Spec.DataService
	metadata := obj.(*v1.QuobyteService).Spec.MetadataService
	// api := obj.(*v1.QuobyteService).Spec.APIService
	syncQuobyteVersion("registry", registry.Image)
	syncQuobyteVersion("data", data.Image)
	syncQuobyteVersion("metadata", metadata.Image)
	resourcehandler.LabelNodes(registry.Nodes, "add", "quobyte_registry")
	resourcehandler.LabelNodes(data.Nodes, "add", "quobyte_data")
	resourcehandler.LabelNodes(metadata.Nodes, "add", "quobyte_metadata")
	keepServiceNodesInSync(registry.Nodes, data.Nodes, metadata.Nodes)
}

func QueryPodsUpToDateness(K8sAPIClient *kubernetes.Clientset) {
	resourcehandler.KubernetesClient = K8sAPIClient
	queryClientPodUpToDateness()
	queryServicesPodUpToDateness()
}

func queryServicesPodUpToDateness() {
	queryPodUpToDateness("registry")
	queryPodUpToDateness("data")
	queryPodUpToDateness("metadata")
}

func queryClientPodUpToDateness() {
	queryPodUpToDateness("client")
}

func queryPodUpToDateness(service string) {
	ds, err := resourcehandler.GetDaemonsetByName(service)
	if err != nil {
		glog.Errorf("Status update failure: Failed to get %s daemonset due to %v", service, err)
		return
	}
	if ds.Status.DesiredNumberScheduled != ds.Status.UpdatedNumberScheduled {
		resourcehandler.ControlledPodUpdate(service, ds.Spec.Template.Spec.Containers[0].Image, false)
	}
}

func syncQuobyteVersion(dsName, image string) {
	// See if daemonset version matches the definition of resource
	deployedDS, err := resourcehandler.GetDaemonsetByName(dsName)
	if err != nil {
		glog.Errorf("Failed to get %s daemonset", dsName)
		return
	}

	if image != deployedDS.Spec.Template.Spec.Containers[0].Image {
		updateDSVersion(dsName, image)
	}
}

func updateDSVersion(dsName, version string) {
	err := resourcehandler.UpdateDaemonSet(dsName, version)
	if err != nil {
		glog.Errorf("Failed to update %s daemonset with updated version %s\n", dsName, version)
	}
}

func handleServicesCRUpdate(old, cur interface{}) {
	oldCr := old.(*v1.QuobyteService)
	curCr := cur.(*v1.QuobyteService)

	oldRegistry := oldCr.Spec.RegistryService
	newRegistry := curCr.Spec.RegistryService
	oldData := oldCr.Spec.DataService
	newData := curCr.Spec.DataService
	oldMetadata := oldCr.Spec.MetadataService
	newMetadata := curCr.Spec.MetadataService

	if !reflect.DeepEqual(oldCr, curCr) {

		// oldAPI := oldCr.Spec.APIService
		// newAPI := curCr.Spec.APIService

		if !reflect.DeepEqual(oldRegistry.Nodes, newRegistry.Nodes) {
			glog.Infof("Registry nodes changed: old %v and new %v", oldRegistry.Nodes, newRegistry.Nodes)
			handleNodeChanges(oldCr, curCr, oldRegistry.Nodes, newRegistry.Nodes, "quobyte_registry")
		}

		if !reflect.DeepEqual(oldData.Nodes, newData.Nodes) {
			glog.Infof("Data service nodes changed: old %v and new %v", oldData.Nodes, newData.Nodes)
			handleNodeChanges(oldCr, curCr, oldData.Nodes, newData.Nodes, "quobyte_data")
		}

		if !reflect.DeepEqual(oldMetadata.Nodes, newMetadata.Nodes) {
			glog.Infof("Metadata service nodes changed: old %v and new %v", oldMetadata.Nodes, newMetadata.Nodes)
			handleNodeChanges(oldCr, curCr, oldMetadata.Nodes, newMetadata.Nodes, "quobyte_metadata")
		}

		// TODO : update node by node. currently service by service
		if oldRegistry.Image != newRegistry.Image {
			svc := "registry"
			updateDSVersion(svc, newRegistry.Image)
			if newRegistry.RollingUpdate {
				resourcehandler.ControlledPodUpdate(svc, newRegistry.Image, true)
			} else {
				printManualUpdateMessage(svc)
			}
		}
		if oldData.Image != newData.Image {
			svc := "data"
			updateDSVersion(svc, newData.Image)
			if newData.RollingUpdate {
				resourcehandler.ControlledPodUpdate(svc, newData.Image, true)
			} else {
				printManualUpdateMessage(svc)
			}
		}
		if oldMetadata.Image != newMetadata.Image {
			svc := "metadata"
			updateDSVersion(svc, newMetadata.Image)
			if newMetadata.RollingUpdate {
				resourcehandler.ControlledPodUpdate(svc, newMetadata.Image, true)
			} else {
				printManualUpdateMessage(svc)
			}
		}
	}
	keepServiceNodesInSync(newRegistry.Nodes, newData.Nodes, newMetadata.Nodes)
	// queryServicesPodUpToDateness()
}

func printManualUpdateMessage(svc string) {
	glog.Infof("****Rolling update disabled for %s service. Requires manual deletion of pods to update the Quobyte version.****", svc)
}

func handleServicesCRDelete(definition interface{}) {
	servicesSpec := definition.(*v1.QuobyteService).Spec
	regNodes := servicesSpec.RegistryService.Nodes
	dataNodes := servicesSpec.DataService.Nodes
	metaNodes := servicesSpec.MetadataService.Nodes
	resourcehandler.LabelNodes(regNodes, "remove", "quobyte_registry")
	resourcehandler.LabelNodes(dataNodes, "remove", "quobyte_data")
	resourcehandler.LabelNodes(metaNodes, "remove", "quobyte_metadata")
	registrySelector := "role=registry"
	dataSelector := "role=data"
	metadataSelector := "role=metadata"
	// apiSelector := fmt.Sprintf("version=%s,role=api", resourcehandler.GetVersionFromString(servicesSpec.APIService.Image))
	// resourcehandler.DeletePods(apiSelector)
	resourcehandler.DeletePods(dataSelector)
	resourcehandler.DeletePods(metadataSelector)
	resourcehandler.DeletePods(registrySelector)
	keepServiceNodesInSync(regNodes, dataNodes, metaNodes)
	glog.Info("Removed Quobyte service definition (Pod termination signalled)")
}
