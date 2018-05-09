package resourcehandler

import (
	"fmt"
	quobytev1 "operator/pkg/kubernetes-actors/clientset/versioned"
	"os"

	"github.com/golang/glog"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	crdv1 "operator/pkg/api/quobyte.com/v1"
)

var (
	config           *rest.Config
	QclientConfig    *quobytev1.Clientset
	err              error
	KubernetesClient *kubernetes.Clientset
	APIServerClient  *apiextensionsclient.Clientset
	quobyteNameSpace = "quobyte"
)

//InitClient Initializes kubernetes client for given kubeconfig
func InitClient(kubeconfig string) {
	if kubeconfig == "" { // must be running inside cluster, try to get from env
		kubeconfig = os.Getenv("KUBECONFIG")
	}

	if kubeconfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	} else {
		config, err = rest.InClusterConfig()
	}
	if err != nil {
		glog.Errorf("Error creating kubernetes access client: %s", err)
		os.Exit(1)
	}
	KubernetesClient, err = kubernetes.NewForConfig(config)

	if err != nil {
		panic("Unable to create kubernetes client for the given configuration")
	}
	scheme := runtime.NewScheme()
	if err := crdv1.AddToScheme(scheme); err != nil {
		fmt.Println(err.Error())
	}
	cfg := *config
	cfg.GroupVersion = &crdv1.SchemeGroupVersion
	cfg.APIPath = "/apis"
	QclientConfig, err = quobytev1.NewForConfig(&cfg)
	if err != nil {
		panic("Unable to create /apis client for the given configuration")
	}

	cfg2 := *config
	cfg2.GroupVersion = &schema.GroupVersion{
		Group:   "quobyte.com",
		Version: "v1",
	}
	cfg2.APIPath = "/apis"

	APIServerClient, err = apiextensionsclient.NewForConfig(&cfg2)

	if err != nil {
		panic("Unable to create api extentions client for the given configuration")
	}
	glog.Info("Initialized kubernetes cluster access clients")
}
