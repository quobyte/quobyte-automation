package web

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"operator/pkg/controller"
	"operator/pkg/resourcehandler"
	"operator/pkg/utils"
	"os"
	"strings"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

var (
	K8sAPIClient *kubernetes.Clientset
	// TODO: change to type, map creates sorted tabs in UI
	urlHandler = map[string]fn{
		"Registry Nodes":        registryHandler,
		"Data Nodes":            dataHandler,
		"Metadata Nodes":        metadataHandler,
		"Client Nodes":          clientsHandler,
		"Administrator Actions": statusHandler,
	}
	menuPage = ""
)

type OperatorStatus struct {
	ClientPending   []resourcehandler.ClientUpdateOnHold `json:"clientPending,omitempty"`
	RegistryPending []resourcehandler.ServiceNotUpdated  `json:"registryPending,omitempty"`
	MetaDataPending []resourcehandler.ServiceNotUpdated  `json:"metadataPending,omitempty"`
	DataPending     []resourcehandler.ServiceNotUpdated  `json:"dataPending,omitempty"`
}

type QuobyeDeployedService struct {
	NodeName string
	Pods     *v1.PodList
}
type fn func(w http.ResponseWriter, r *http.Request)

const (
	//port to run webserver for quobyte-operator
	port string = ":7878"
)

func deployedService(nodeLabel, selectorVal string) ([]*QuobyeDeployedService, error) {
	nodes, err := utils.GetQuobyteNodes(nodeLabel, K8sAPIClient)
	if err != nil {
		return nil, err
	}

	servicePods := make([]*QuobyeDeployedService, len(nodes.Items))
	selector := fmt.Sprintf("role=%s", selectorVal)

	for i, node := range nodes.Items {
		nodeName := node.ObjectMeta.Name
		podList, err := GetPods(selector, nodeName)
		if err != nil {
			return nil, err
		}
		servicePods[i] = &QuobyeDeployedService{
			nodeName, podList,
		}
	}
	return servicePods, nil
}

func registryHandler(w http.ResponseWriter, r *http.Request) {
	service, err := deployedService(REGISTRY_LABEL, utils.RegistryService)
	handleNodeResponse(w, r, service, err)
}

func getStatusJSON(w http.ResponseWriter, r *http.Request) {
	service := getStatus()
	json, _ := json.Marshal(service)
	fmt.Fprintf(w, string(json))
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	service, err := deployedService(DATA_LABEL, utils.DataService)
	handleNodeResponse(w, r, service, err)
}

func metadataHandler(w http.ResponseWriter, r *http.Request) {
	service, err := deployedService(METADATA_LABEL, utils.MetadataService)
	handleNodeResponse(w, r, service, err)
}

func clientsHandler(w http.ResponseWriter, r *http.Request) {
	service, err := deployedService(CLIENT_LABEL, utils.ClientService)
	handleNodeResponse(w, r, service, err)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	cleanCacheFiles()
	controller.QueryPodsUpToDateness(K8sAPIClient)
	tmpl := template.Must(template.ParseFiles("/public/html/status.html"))
	tmpl.Execute(w, getStatus())
}

func cleanCacheFiles() {
	os.Remove(utils.CLIENT_STATUS_FILE)
	os.Remove(utils.REGISTRY_STATUS_FILE)
	os.Remove(utils.DATA_STATUS_FILE)
	os.Remove(utils.METADATA_STATUS_FILE)
}

func getStatus() *OperatorStatus {
	raw, err := ioutil.ReadFile(utils.CLIENT_STATUS_FILE)
	var clientStatus []resourcehandler.ClientUpdateOnHold
	if err != nil {
		glog.Errorf("Failed reading status file %v", err)
		clientStatus = nil
	} else {
		err = json.Unmarshal(raw, &clientStatus)
		if err != nil {
			fmt.Printf("Unable to get the current client status")
			glog.Errorf("%v", err)

		}
	}

	return &OperatorStatus{clientStatus, loadServiceStatus(utils.REGISTRY_STATUS_FILE), loadServiceStatus(utils.METADATA_STATUS_FILE), loadServiceStatus(utils.DATA_STATUS_FILE)}

}

func loadServiceStatus(file string) []resourcehandler.ServiceNotUpdated {
	raw, err := ioutil.ReadFile(file)
	if err != nil {
		glog.Errorf("Failed reading status file %v", err)
		return nil
	}
	var serviceStatus []resourcehandler.ServiceNotUpdated
	err = json.Unmarshal(raw, &serviceStatus)
	if err != nil {
		fmt.Printf("Unable to get the current client status")
		glog.Errorf("%v", err)
		return nil
	}
	return serviceStatus

}

func handleNodeResponse(w http.ResponseWriter, r *http.Request, services []*QuobyeDeployedService, err error) {
	if err != nil {
		fmt.Fprintf(w, "Failed getting Quobyte nodes")
		glog.Errorf("%v", err)
		return
	}
	if len(services) == 0 {
		fmt.Fprintf(w, "No Nodes found")
		return
	}

	tmpl := template.Must(template.ParseFiles("/public/html/nodes.html"))
	tmpl.Execute(w, services)
}

func menuHandler(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, ".css") {
		sendCSS(w, r)
		return
	}
	tmpl := template.Must(template.ParseFiles("/public/html/index.html"))
	tmpl.Execute(w, urlHandler)
}

func sendCSS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/css")
	css, _ := ioutil.ReadFile("/public/" + r.URL.Path)
	w.Write(css)
}

//StartWebServer Starts webserver on port
func StartWebServer(apiClient *kubernetes.Clientset) {
	K8sAPIClient = apiClient
	glog.Infof("Starting server on port: %s", port)
	http.HandleFunc("/", menuHandler)
	// http.HandleFunc("/Get JSON", getStatusJSON)
	for url, handleFn := range urlHandler {
		http.HandleFunc("/"+url, handleFn)
	}
	http.HandleFunc("/Status JSON", getStatusJSON)
	http.ListenAndServe(port, nil)
}
