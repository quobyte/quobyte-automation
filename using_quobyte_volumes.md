# Volume Access

Quobyte volumes can be accessed from Pods in multiple ways. Either, via
the QuobyteVolumeSource like in `deploy/example-pod.yaml`

```yaml
volumes:
- name: quobytevolume
  quobyte:
    registry: ignored:7861 # Unused string required for API compatibility
    volume: testVolume
    readOnly: false
    user: username
    group: groupname
```

or as a PersistentVolumeClaim, which is defined in the same namespace, where
the accessing pod is running. The claim (volumes/claim.json)

```json
{
  "kind": "PersistentVolumeClaim",
  "apiVersion": "v1",
  "metadata": {
    "name": "test"
  },
  "spec": {
    "accessModes": [
      "ReadWriteOnce"
    ],
    "resources": {
      "requests": {
        "storage": "3Gi"
      }
    },
    "storageClassName": "base"
  }
}
```

```bash
$ kubectl -n quobyte create -f volumes/claim.json
```

is mounted in the Pod like (see volumes/example-pod.yaml):

```yaml
volumes:
  - name: quobytepvc
    persistentVolumeClaim:
      claimName: test
```

The PersistentVolumeClaim is bound to a PersistentVolume which is created by an
administrator for existing Quobyte Volumes or dynamically by the StorageClass
(volumes/storageclass.yaml).

If a Quobyte Volume already exists, the administrator can make it available
as a PersistentVolume by creating a resource (volumes/pv.yaml).
Please note that Quobyte currently ignores the required capacity.storage field,
since its using internal quota mechanisms.

```yaml
kind: PersistentVolume
apiVersion: v1
metadata:
  name: test
  labels:
    type: quobyte
spec:
  capacity:
    storage: 1Gi
  accessModes:
    - ReadWriteOnce
  storageClassName: "base"
  quobyte:
    registry: ignored:7861 # Unused string required for API compatibility
    volume: test
    readOnly: false
    user: username
    group: groupname
```

```bash
$ kubectl -n quobyte create -f volumes/pv.yaml
```

For dynamic provisioning, a StorageClass is created, which manages the
lifecycle of Quobyte volumes.
Each provisioner is bound to a Quobyte tenant, which is specified in the 'quobyteTenant' field.
The UUID of the tenant can be found in the Webconsole.
For Kubernetes > 1.10, the 'quobyteTenant' can specify the name of the tenant.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
   name: base
provisioner: kubernetes.io/quobyte
parameters:
    quobyteAPIServer: "http://api.quobyte:7860"
    registry: "registry.quobyte:7861"
    adminSecretName: "quobyte-admin-secret"
    adminSecretNamespace: "kube-system"
    user: "username"
    group: "groupname"
    quobyteConfig: "BASE"
    quobyteTenant: "uuid of tenant"
    createQuota: "False"
```

To enable it, first create the Quobyte admin secret according to your API
credentials in volumes/quobyte-admin-secret.yaml (defaults to admin:quobyte):
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: quobyte-admin-secret
type: "kubernetes.io/quobyte"
data:
  password: cXVvYnl0ZQ==
  user: YWRtaW4=
type: kubernetes.io/quobyte
```

The password and user strings are base64 encoded and can be created like
`echo "quobyte" | base64`.

```bash
$ kubectl -n kube-system create -f volumes/quobyte-admin-secret.yaml
$ kubectl -n quobyte create -f volumes/storageclass.yaml
```

## Quobyte Tenants and Kubernetes Namespaces

Quobyte supports multiple tenants and provides a secure mapping of containers to
 users known in Quobyte.
For a longer read, please see the article on the Quobyte blog:
[The State of Secure Storage Access in Container Infrastructures](https://www.quobyte.com/blog/2017/03/17/the-state-of-secure-storage-access-in-container-infrastructures/)

The Kubernetes deployments of the Quobyte client use the `--allow-usermapping-in-volumename`, which allows to map all accesses to the
volume to a particular user/group, independent of the accessing user.
If you specify `user#group@volume_name` instead of just the volumename,
the Quobyte client will map all storage accesses to the `user:group`.
For example, if your container runs internally with id 0, or you have multiple
arbitrary user ids in your containers, all files in the Quobyte volume will be
owned by `user:group`.

## Further Readings

- https://github.com/kubernetes/examples/tree/master/staging/volumes/quobyte

- https://kubernetes.io/docs/tasks/configure-pod-container/configure-persistent-volume-storage/#create-a-persistentvolume

- https://kubernetes.io/docs/concepts/storage/storage-classes/#quobyte
