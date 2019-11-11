# NetworkAttachmentDefinition CRD Client

Based on https://github.com/openshift-evangelists/crd-code-generation

## Getting Started

First register the custom resource definition:

```
kubectl apply -f artifacts/network-crd.yaml
```

For old kubernetes version (<1.16), please use `artifacts/networks-crd-v1beta1.yaml`:

```
kubectl apply -f artifacts/networks-crd-v1beta1.yaml
```

Then add an example of the `NetworkAttachmentDefinition` kind:

```
kubectl apply -f artifacts/my-network.yaml
```

Finally build and run the example:

```
go build
./example -kubeconfig ~/.kube/config
```
