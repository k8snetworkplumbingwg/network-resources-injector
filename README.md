# Network Resources Injector

[![Weekly minutes](https://img.shields.io/badge/Weekly%20Meeting%20Minutes-Mon%203pm%20GMT-blue.svg?style=plastic)](https://docs.google.com/document/d/1sJQMHbxZdeYJPgAWK1aSt6yzZ4K_8es7woVIrwinVwI)

Network Resources Injector is a Kubernetes Dynamic Admission Controller application that provides functionality of patching Kubernetes pod specifications with requests and limits of custom network resources (managed by device plugins such as [k8snetworkplumbingwg/sriov-network-device-plugin](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin)).

## Getting started

To quickly build and deploy admission controller run:
```
make image
kubectl apply -f deployments/auth.yaml \
              -f deployments/server.yaml
```
For full installation and troubleshooting steps please see [Installation guide](docs/installation.md).

## Network resources injection example

To see mutating webhook in action you're going to need to add custom resources to your Kubernetes node. In real life scenarios you're going to use network resources managed by network devices plugins, such as [k8snetworkplumbingwg/sriov-network-device-plugin](https://github.com/k8snetworkplumbingwg/sriov-network-device-plugin).
There should be [net-attach-def CRD](https://github.com/intel/multus-cni/blob/master/examples/crd.yml) already created before you start.
In a terminal window start proxy, so that you can easily send HTTP requests to the Kubernetes API server:
```
kubectl proxy
```
In another terminal window, execute below command to add 4 `example.com/foo` resources. Remember to edit `<node-name>` to match your cluster environment.

```
curl -s --header "Content-Type: application/json-patch+json" \
     --request PATCH \
     --data '[{"op": "add", "path": "/status/capacity/example.com~1foo", "value": "4"}]' \
     http://localhost:8001/api/v1/nodes/<node-name>/status >/dev/null
```
Next, you need to create a net-attach-def linked to this `example.com/foo` resource. To achieve that execute below command:
```
cat <<EOF | kubectl create -f -
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  annotations:
    k8s.v1.cni.cncf.io/resourceName: example.com/foo
  name: foo-network
  namespace: default
spec:
  config: |
    {
      "cniVersion": "0.3.0",
      "name": "foo-network",
      "type": "bridge",
      "bridge": "br0",
      "isGateway": true,
      "ipam":
      {
        "type": "host-local",
        "subnet": "172.36.0.0/24",
        "dataDir": "/mnt/cluster-ipam"
      }
    }
EOF
```
Finally, schedule a pod that will take advantage of automated network resources injection. Use below example, to attach 2 `foo-network` networks and inject resources accordingly.
```
cat <<EOF | kubectl create -f -
apiVersion: v1
kind: Pod
metadata:
  name: webhook-demo
  annotations:
    k8s.v1.cni.cncf.io/networks: foo-network, foo-network
spec:
  containers:
  - image: busybox
    resources:
    command: ["tail", "-f", "/dev/null"]
    imagePullPolicy: IfNotPresent
    name: busybox
  restartPolicy: Always
EOF
```
Now verify that resources requests/limits have been properly injected into the first container in the pod. If you have `jq` installed run this command:
```
kubectl get pod webhook-demo -o json | jq .spec.containers[0].resources
```
Expected output showing injected resources in the pod spec, depsite the fact that we have only defined `k8s.v1.cni.cncf.io/networks` annotation.
```
{
  "limits": {
    "example.com/foo": "2"
  },
  "requests": {
    "example.com/foo": "2"
  }
}
```
Alternatively, grep output of kubectl to get the same information.
```
kubectl get pod webhook-demo -o yaml | grep resources -A4
    resources:
      limits:
        example.com/foo: "2"
      requests:
        example.com/foo: "2"
```
As the last step perform cleanup by removing net-attach-def, pod and custom `example.com/foo` resources. To do that, simply run:
```
curl --header "Content-Type: application/json-patch+json" \
     --request PATCH \
     --data '[{"op": "remove", "path": "/status/capacity/example.com~1foo"}]' \
     http://localhost:8001/api/v1/nodes/<node-name>/status
kubectl delete net-attach-def foo-network
kubectl delete pod webhook-demo
```

## Vendoring
To create the vendor folder invoke the following which will create a vendor folder.
```bash
make vendor
```

## Security
### Disable adding client CAs to server TLS endpoint
If you wish to not add any client CAs to the servers TLS endpoint, add ```--insecure``` flag to webhook binary arguments (See [server.yaml](deployments/server.yaml)).

### Client CAs
By default, we consume the client CA from the Kubernetes service account secrets directory ```/var/run/secrets/kubernetes.io/serviceaccount/```.
If you wish to consume a client CA from a different location, please specify flag ```--client-ca``` with a valid path. If you wish to add more than one client CA, repeat this flag multiple times. If ```--client-ca``` is defined, the default client CA from the service account secrets directory will not be consumed.

## Additional features
### Expose Hugepages via Downward API
In Kubernetes 1.20, an alpha feature was added to expose the requested hugepages to the container via the Downward API.
Being alpha, this feature is disabled in Kubernetes by default.
If enabled when Kubernetes is deployed via `FEATURE_GATES="DownwardAPIHugePages=true"`, then Network Resource Injector can be used to mutate the pod spec to publish the hugepage data to the container. To enable this functionality in Network Resource Injector, add ```--injectHugepageDownApi``` flag to webhook binary arguments (See [server.yaml](deployments/server.yaml)).

> NOTE: Please note that the Network Resource Injector does not add hugepage resources to the POD specification. It means that user has to explicitly add it. This feature only exposes it to Downward API. More information about hugepages can be found within Kubernetes [specification](https://kubernetes.io/docs/tasks/manage-hugepages/scheduling-hugepages/). Snippet of how to request hugepage resources in pod spec:
```
spec:
  containers:
  - image: busybox
    resources:
      limits:
        hugepages-1Gi: 2Gi
        memory: 2Gi
      requests:
        hugepages-1Gi: 2Gi
        memory: 2Gi
```

Like the other Downward API provided data, hugepage information for a pod can be located by an application at the path `/etc/podnetinfo/` in the container's file system.
This directory will contain the request and limit information for 1Gi/2Mb.

1Gi Hugepages:
```
    Requests: /etc/podnetinfo/hugepages_1G_request_${CONTAINER_NAME}
    Limits: /etc/podnetinfo/hugepages_1G_limit_${CONTAINER_NAME}
```
2Mb: Hugepages:
```
    Requests: /etc/podnetinfo/hugepages_2M_request_${CONTAINER_NAME}
    Limits: /etc/podnetinfo/hugepages_2M_limit_${CONTAINER_NAME}
```

> NOTE: To aid the application, when hugepage fields are being requested via the Downward API, Network Resource Injector also mutates the pod spec to add the environment variable `CONTAINER_NAME` with the container's name applied.

### Node Selector
If a ```NetworkAttachmentDefinition``` CR annotation ```k8s.v1.cni.cncf.io/nodeSelector``` is present and a pod utilizes this network, Network Resources Injector will add this node selection constraint into the pod spec field ```nodeSelector```. Injecting a single node selector label is currently supported.

Example:
```yaml
apiVersion: k8s.cni.cncf.io/v1
kind: NetworkAttachmentDefinition
metadata:
  name: test-network
  annotations:
    k8s.v1.cni.cncf.io/nodeSelector: master=eno3
spec:
  config: '{
  "cniVersion": "0.3.1",
  "type": "macvlan",
  "master": "eno3",
  "mode": "bridge",
  }'
...
```
Pod spec after modification by Network Resources Injector:
```yaml
apiVersion: v1
kind: Pod
metadata:
  name: testpod
  annotations:
    k8s.v1.cni.cncf.io/networks: test-network
spec:
 ..
 nodeSelector:
   master: eno3
```

### User Defined Injections

User Defined injections allows user to define additional injections (besides what's supported in NRI, such as ResourceName, Downward API volumes etc) in Kubernetes ConfigMap and request additional injection for individual pod based on pod label. Currently user defined injection only support injecting pod annotations.

In order to use this feature, user needs to create the user defined injection ConfigMap with name `nri-user-defined-injections` in the namespace where NRI was deployed in (`kube-system` namespace is used when there is no `NAMESPACE` environment variable passed to NRI). The data entry in ConfigMap is in the format of key:value pair. Key is a user defined label that will be used to match with pod labels, Value is the actual injection in the format as defined by [RFC6902](https://tools.ietf.org/html/rfc6902) that will be applied to pod manifest. NRI would listen to the creation/update/deletion of this ConfigMap and update its internal data structure every 30 seconds so that subsquential creation of pods will be evaluated against the latest user defined injections.

Metadata.Annotations in Pod definition is the only supported field for customization, whose `path` should be "/metadata/annotations".

Below is an example of user defined injection ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: nri-user-defined-injections
  namespace: kube-system
data:
  feature.pod.kubernetes.io/sriov-network: '{"op": "add", "path": "/metadata/annotations", "value": {"k8s.v1.cni.cncf.io/networks": "sriov-net-attach-def"}}'
```

`feature.pod.kubernetes.io/sriov-network` is a user defined label to request additional networks. Every pod that contains this label with a value set to `"true"` will be applied with the patch that's defined in the following json string.

`'{"op": "add", "path": "/metadata/annotations", "value": {"k8s.v1.cni.cncf.io/networks": "sriov-net -attach-def"}}` defines how/where/what the patch shall be applied.

`"op": "add"` is the action for user defined injection. In above case, it requests to add the `value` to `path` in pod manifests.

`"path": "/metadata/annotations"` is the path in pod manifest to be updated.

`"value": {"k8s.v1.cni.cncf.io/networks": "sriov-net-attach-def"}}` is the value to be updated in the given `path`.

For a pod to request user defined injection, one of its labels shall match with the labels defined in user defined injection ConfigMap.
For example, with the below pod manifest:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: testpod
  labels:
   feature.pod.kubernetes.io/sriov-network: "true"
spec:
  containers:
  - name: app
    image: centos:7
    command: [ "/bin/bash", "-c", "--" ]
    args: [ "while true; do sleep 300000; done;" ]
```

NRI would update pod manifest to:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: testpod
  labels:
   feature.pod.kubernetes.io/sriov-network: "true"
  annotations:
    k8s.v1.cni.cncf.io/networks: sriov-net-attach-def
spec:
  containers:
  - name: app
    image: centos:7
    command: [ "/bin/bash", "-c", "--" ]
    args: [ "while true; do sleep 300000; done;" ]
```

## Test
### Unit tests

```
$ make test
```

### E2E tests using Kubernetes in Docker (KinD)
Deploy KinD and run tests

```
$ make e2e
```

Cleanup KinD deployment

```
make e2e-clean
```

## Contact Us

For any questions about Network Resources Injector, feel free to ask a question in #network-resources-injector in [NPWG slack](https://npwg-team.slack.com/), or open up a GitHub issue. Request an invite to NPWG slack [here](https://intel-corp.herokuapp.com/).
