package web

import (
	"fmt"
	"net/http"
	"operator/pkg/resourcehandler"
)

const (
	//port to run webserver for quobyte-operator
	port string = ":7878"
)

func nodeListHandler(w http.ResponseWriter, r *http.Request) {
	nodes, err := resourcehandler.GetQuobyteNodes()
	if err != nil {
		panic(err)
	}
	for _, node := range nodes.Items {
		fmt.Fprintf(w, "Node: %s\n", node.Name)
	}
}

func menuHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "hello menu")
}

//StartWebServer Starts webserver on port
func StartWebServer() {
	fmt.Printf("Starting server on port: %s", port)
	http.HandleFunc("/listNodes", nodeListHandler)
	http.HandleFunc("/menu", menuHandler)
	http.ListenAndServe(port, nil)
}
