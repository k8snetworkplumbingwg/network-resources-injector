# Installation guide

## Building Docker image
Go to the root directory of the Network Resources Injector and build image:
```
cd $GOPATH/src/github.com/intel/network-resources-injector
make
```

## Deploying webhook application
Create Service Account for network resources injector mutating admission webhook and webhook installer and apply RBAC rules to created account:
```
kubectl apply -f deployments/auth.yaml
```

Next step creates Kubernetes pod. Init container creates all resources required to run webhook:
* TLS key and certificate signed with Kubernetes CA
* mutating webhook configuration
* service to expose webhook deployment to the API server

After successful completion of the init container work, the actual webhook server application container is started.

Execute command:
```
kubectl apply -f deployments/server.yaml
```

*Note: Verify that Kubernetes controller manager has --cluster-signing-cert-file and --cluster-signing-key-file parameters set to paths to your CA keypair
to make sure that Certificates API is enabled in order to generate certificate signed by cluster CA.
More details about TLS certificates management in a cluster available [here](https://kubernetes.io/docs/tasks/tls/managing-tls-in-a-cluster/).*
