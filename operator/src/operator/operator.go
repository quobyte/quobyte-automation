package main

import (
	"flag"
	"operator/pkg/controller"
	"operator/pkg/resourcehandler"
	"operator/pkg/web"
	"os"

	"github.com/golang/glog"
)

func main() {
	kubeconfig := ""
	flag.StringVar(&kubeconfig, "kubeconfig", kubeconfig, "kubeconfig file not found")
	flag.Parse()
	flag.Lookup("logtostderr").Value.Set("true")
	flag.Lookup("v").Value.Set("1")
	glog.Info("Starting Quobyte operator")
	resourcehandler.InitClient(kubeconfig)
	if resourcehandler.KubernetesClient != nil {
		glog.Info("Starting operator UI")
		go web.StartWebServer(resourcehandler.KubernetesClient)
	}

	err := resourcehandler.CreateAllQuobyteCrd()
	if err != nil {
		glog.Errorf("Terminating operator due to: \n %v", err)
		os.Exit(1)
	}
	glog.Info("Starting operator")
	controller.Start(resourcehandler.QclientConfig)
}
