# Quick Start Guide to use Quobyte from Kubernetes

## Prerequisites
If you use Docker 1.12 or older, you need to  add the MountFlags parameter.
For newer versions of Docker, this is not required anymore.

Ensure MountFlags are shared or not set:
```bash
$ systemctl cat docker.service | grep MountFlags=shared
```

If not you can set the MountFlags as shared with the following commands:
```bash
$ cat << EOF > /etc/systemd/system/docker.service.d/slave-mount-flags.conf
[Service]
MountFlags=shared
EOF

$ systemctl daemon-reload && systemctl restart docker
```

## Client setup
This guide assumes that you have a dedicated Quobyte server running and you
want to provide access to Quobyte volumes to pods running in Kubernetes.

To access a Quobyte volume a pod has to run on a Kubernetes node which has a
Quobyte client running. The client runs inside of a Pod and makes the Quobyte
storage accessible to other pods.

Quobyte clients run in the quobyte namespace.
```bash
$ cd deploy
$ kubectl create -f quobyte-ns.yaml
```

To connect to Quobyte, the client needs to resolve the address of the registry.
It is configured in the client-ds.yaml DaemonSet definition:
```yaml
env:
  - name: QUOBYTE_REGISTRY
    value: registry.quobyte
```
If you have a QNS (Quobyte Name Service), the value for QUOBYTE_REGISTRY might
look like `<qnsid>.myquobyte.net`

If you have a certificate for the client, it is stored as a Secret and
mounted into the client Pod as client.cfg.

First create a file that contains only the certificate information
(<ca>, <cert>, and <key> blocks) and store it as a secret.
```bash
$ kubectl -n quobyte create secret generic client-config --from-file /tmp/client.cfg
```

```bash
$ kubectl -n quobyte create -f client-ds.yaml
or
$ kubectl -n quobyte create -f client-certificate-ds.yaml
```

The deployed DaemonSet starts client pods on all nodes marked as `quobyte_client`.
This can either be done manually, or by using the Quobyte operator.

```bash
$ kubectl label nodes <node-1> <node-n> quobyte_client="true"
```

When the client pod is up and running, you should see a mount point on the Kubernetes node
at `/var/lib/kubelet/plugins/kubernetes.io~quobyte`.

Please note: Only those nodes labeled with the 'quobyte_client' label and hence
are running the Quobyte client can provide access to Quobyte storage to other pods
running on the node.

##Benchmarking

For easy testing and benchmarking we provide a fio-container which uses
Quobyte volumes. By default, it will start writing to volume `fio-test`

```bash
$ kubectl create -f fio-benchmark-ds.yaml
```
This will start a single Pod on a node which is marked as quobyte_client.
The container is designed to put load on the volume, so you can scale it:

```bash
$ kubectl scale --replicas=100 deployment fio-benchmark
```

# Volume Access
See [using_quobyte_volumes.md](using_quobyte_volumes.md)
