# Bootstrap and Operate a Quobyte Cluster in Kubernetes with the Operator
This guide shows you how to bootstrap a Quobyte cluster on a set of kubernetes
which have some empty storage devices ready to be used for a distributed storage system. The operator simplifies the cluster bootstrap and management, and the latest Quobyte features let you format and set up the cluster from the web console.
Can it be any easier and faster?

## Prerequisites
- Kubernetes 1.9 is fully supported by the operator
- To use Quobyte 2.0 features like automatically mounting Quobyte devices,
  or the formatting and preparation of unformatted devices requires the *mountPropagation* kubernetes feature, which is a gated feature in kubernetes 1.9
  and comes in beta in kubernetes 1.10. This guide assumes that you have the *mountPropagation* feature enabled on your cluster.
- A cluster which consists of at least 4 nodes, with 2 unformatted devices each.
  In this guide we will refer to the nodes as node1 to node4.

## Deploy Operator
Create the *quobyte* namespace. This is where the operator and the cluster lives.
```bash
kubectl create -f deploy/quobyte-ns.yaml
```
Quobyte runs best with 3 replicas of the registry, where we require 1 bootstrapped registry. To make the setup as easy as possible, we defined an ephemeral registry, which is used to bootstrap the cluster. The final cluster will have registry devices on nodes 2, 3, and 4, so we will use `node1` for bootstrap.

The `quobyte-config.yaml` file provides a `registry.bootstrap_node` option and allows to fine tune the memory limits for the services. Edit the file to point to your bootstrap registry.

```yaml
  registry.bootstrap_node: "node1"
```

Now install the operator to the quobyte namespace.
```bash
cp quobyte-config.yaml.tmp quobyte-config.yaml
kubectl -n quobyte create -f deploy/quobyte-config.yaml
kubectl -n quobyte create -f deploy/operator.yaml
```

## Configure Quobyte Services
To run the Quobyte services in kubernetes, first copy the `services-config.yaml.tmp` to `services-config.yaml.tmp` then edit the `services-config.yaml` file,
and determine which node should run which services.

We chose node4 to be the bootstrap registry, but we need to define 3 other nodes to persist the fully replicated cluster. We also recommend to start at least 3 metadata services and data services on all nodes which contain devices which should store your valuable information. Edit the `services-config.yaml` to match your cluster:

```yaml
  registry:
    image: quay.io/quobyte/quobyte-server:2
    rolling_updates_enabled: true
    nodes:
      - node1 # will become the ephemeral bootstrap node
  metadata:
    image: quay.io/quobyte/quobyte-server:2
    rolling_updates_enabled: true
    nodes:
      - node1
      - node2
      - node3
      - node4
  data:
    image: quay.io/quobyte/quobyte-server:2
    rolling_updates_enabled: true
    nodes:
      - node1
      - node2
      - node3
      - node4
```

When the services are installed, the operator will start to deploy the services to the target nodes.
```
kubectl -n quobyte create -f deploy/services.yaml
kubectl -n quobyte create -f deploy/services-config.yaml
kubectl -n quobyte create -f deploy/qmgmt-pod.yaml

kubectl -n quobyte get pods -o wide -w
```
Now you should see all configured pods running.
You will also see a *qmgmt-pod* and a *webconsole* running.
The qmgmt-pod gives you full cli access to the cluster. Lets check that the
primary registry is running:

```bash
kubectl -n quobyte exec -it qmgmt-pod -- qmgmt -u api registry list
```

Now let's list all unformatted devices which the data and metadata services found
and could be formatted now.

```bash
kubectl -n quobyte exec -it qmgmt-pod -- qmgmt -u api device list-unformatted
```

You can either proceed to set up the devices with qmgmt or jump over to the webconsole to get some visual support.

Unless you already have set up an ingress to access the service, you can acceess the console with a port forward
```
kubectl -n quobyte port-forward $(kubectl -n quobyte get po -l role=webconsole -o name) 8080:8080
```
Then point your browser to http://localhost:8080 and follow the setup wizard.

The [Devices tab](http://localhost:8080/#DeviceListState:) will show you all unformatted devices. Please note that even if multiple services are running on the same node, only one of the Quobyte pods will be responsible to mount and format devices.

## Make the Quobyte Cluster Persistent
The ephemereal registry is great for bootstrapping and trying out a Quobyte cluster.
So if you're just interested in a demo, you can safely skip this chapter and just
use a single registry to run your cluster. But please note, once the registry pod is terminated,
the Quobyte cluster becomes unsuable.

To make the cluster persistent, we first need to create 3 registry devices.
There must be only one registry device per registry service, so choose one device from each of the other services and create registry devices on them.
A maintenance task will run and format and set up the devices. Give the webconsole some seconds to retrieve the last system state and the devices will show up as
unassociated devices.

Now let's spin up our three target registries.
Edit the services-config.yaml again and add nodes 2 to 4 as registries.

Edit the `services-config.yaml` to match your cluster

```yaml
  registry:
    image: quay.io/quobyte/quobyte-server:2
    rolling_updates_enabled: true
    nodes:
      - node1 # will become the ephemeral bootstrap node
      - node2
      - node3
      - node4
```

An update to the services-config CRD triggers the operator, which will start the
registries then.

```bash
kubectl -n quobyte apply -f deploy/services-config.yaml
```

Wait until the pods are running and check that Quobyte found the devices.

```bash
kubectl -n quobyte exec -it qmgmt-pod -- qmgmt -u api device list

Id  Host            Mode    Disk Used  Disk Avail  Services  LED Mode  Tags  
1   registry-vfz59  ONLINE  4 GB       40 GB       REGISTRY  OFF       hdd   
2   registry-f7szc  ONLINE  34 MB      21 GB       REGISTRY  OFF       hdd   
3   registry-zq2vh  ONLINE  34 MB      21 GB       REGISTRY  OFF       hdd   
4   registry-4xxkk  ONLINE  34 MB      21 GB       REGISTRY  OFF       hdd   
```

We see a total of 4 registry devices, but the registry will only use 3 of them.
```bash
kubectl -n quobyte exec -it qmgmt-pod -- qmgmt -u api registry list

Primary  Id  Host            Mode    
-        3   registry-zq2vh  ONLINE  
-        4   registry-4xxkk  ONLINE  
1        1   registry-vfz59  ONLINE  
```

If we see 3 ONLINE registries, the work of the ephemeral bootstrap node is done
and it is safe to delete it. So remove it from the `services-config.yaml`
```yaml
  registry:
    image: quay.io/quobyte/quobyte-server:2
    rolling_updates_enabled: true
    nodes:
      - node2
      - node3
      - node4
```
and update the service-config. The operator will then terminate the ephemeral registry.
```bash
kubectl -n quobyte apply -f deploy/services-config.yaml
```
Wait some seconds and check that all 3 persisted registries are ONLINE.
```bash
kubectl -n quobyte exec -it qmgmt-pod -- qmgmt -u api registry list
P
rimary  Id  Host            Mode    
-        1   registry-vfz59  ONLINE  
-        3   registry-zq2vh  ONLINE  
1        4   registry-4xxkk  ONLINE  
```

As a last step, you should decommission the ephemeral device, since it will never come back.
```bash
kubectl -n quobyte exec -it qmgmt-pod -- qmgmt -u api device update status 1 DECOMMISSIONED
```

## Create Data and Metadata Devices
Now we need some data and metadata devices to actually store data.
From the webconsole, either format the remaining devices according to your needs,
or choose a device and *Set devices types* to add Data or Metadata contents to the device.

Now you have a fully working Quobyte cluster. For further configuration and creation of volumes, please refer to the Quobyte documentation.

# Deploy Quobyte Clients
The operator can deploy and manage Quobyte clients - which serve the volumes to your application pods. Every kubernetes node which should provide access to Quobyte storage, has to run a Quobyte client pod.

If the operator finds a client CRD, it will start to deploy the according pods.
First copy the `client-config.yaml.tmp` to `client-config.yaml` then edit it.

```yaml
spec:
    image: quay.io/quobyte/quobyte-client:2
    rolling_updates_enabled: true
    nodes:
      - node1
      - node2
      - node3
      - node4
```

```bash
kubectl -n quobyte create -f deploy/client-config.yaml
kubectl -n quobyte create -f deploy/client.yaml
```

Once the client-config is created, you should see pods being started on the
desired hosts.

If you add or remove clients, edit the client-config.yaml and update it with

```
kubectl -n quobyte apply -f deploy/-config.yaml
```

When the clients are ready, you can start using Quobyte volumes in your pods.
Please have a look at [Volume Access](../using_quobyte_volumes.md) for examples.

# Rolling Updates
Note: Rolling updates are currently in Beta.
The operator supports rolling updates. When you change the container version
in the client-config.yaml or services-config.yaml, and rolling updates are enabled, the operator will upgrade one node after the other.

Quobyte service containers are updated with careful timing between pod restarts, to always ensure availability of the Quobyte services.

All pods from all other namespaces can access Quobyte volumes which are managed by the client. Since a client update requires a
pod restart, all other pods on the same node, which currently access a Quobyte volume, need to be stopped.
It's not a good idea to give an operator full permission to drain a full node, we decided to go for a defensive mode.
For every node to upgrade, the operator checks for other pods with Quobyte volumes mounted. If no pods are found, the client is restarted immediately.
If pods are found, they are listed on the operator's status page. The operator also supports to retrieve its status as json.
The administrator will then need to manually stop or drain the pods.   

The operator comes with a service and a status page. With kubectl, you can reach it on http://localhost:7878
```bash
kubectl -n quobyte port-forward $(kubectl -n quobyte get po -l role=quobyte-operator -o name) 7878:7878
```

# Uninstall Quobyte with Operator
If you want to remove all services or clients, remove the config files, before
you delete the deployments or the operator. This will terminate the scheduled pods and remove the all labels, which the operator applied to any nodes.
```
kubectl -n quobyte delete -f deploy/services-config.yaml
or
kubectl -n quobyte delete -f deploy/client-config.yaml
```

# Build Operator from Source

## Requirements
1. golang 1.8+
2. glide for package management
3. docker

## Build
1. Clone the repository.
```
git clone git@github.com:quobyte/kubernetes.git quobyte-kubernetes
```
2. Compile and build binary from source.
```
cd quobyte-kubernetes/operator
export GOPATH=$(pwd)
cd src/operator
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o operator .
```
If you're building for the first time after clone run ``glide install --strip-vendor`` to get the dependencies.

3. To run operator outside cluster (skip to 4 to run operator inside cluster)
```
./operator --kubeconfig <kuberenetes-admin-conf>
```
  Follow [Deploy clients](#deploy-clients), and you can skip step 3 of deploy clients.

4. Build Docker container
```
sudo docker build -t operator -f Dockerfile.scratch .
sudo docker run -it operator
```
5. Get the ``CONTAINER ID`` of the operator.
```
sudo docker ps -l
```
6. Commit the container.
```
sudo docker commit <CONTAINER ID>  <Docker-repository-url>
```
7. Push to the container repository.
```
sudo docker push <Docker-repository-url>
```
8. Edit ``operator.yaml`` and point ``quobyte-operator`` container image to the docker image.
