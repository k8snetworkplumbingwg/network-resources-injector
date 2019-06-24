# Network Resources Injector

Network Resources Injector is a Kubernetes Dynamic Admission Controller application that provides functionality of patching Kubernetes pod specifications with requests and limits of custom network resources (managed by device plugins such as [intel/sriov-network-device-plugin](https://github.com/intel/sriov-network-device-plugin)).

## Getting started

To quickly build and deploy admission controller run:
```
make image
kubectl apply -f deployments/auth.yaml \
              -f deployments/server.yaml
```
For full installation and troubleshooting steps please see [Installation guide](docs/installation.md).

## Network resources injection example

To see mutating webhook in action you're going to need to add custom resources to your Kubernetes node. In real life scenarios you're going to use network resources managed by network devices plugins, such as [intel/sriov-network-device-plugin](https://github.com/intel/sriov-network-device-plugin).
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
