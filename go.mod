module github.com/intel/network-resources-injector

go 1.13

require (
	github.com/cloudflare/cfssl v1.4.1
	github.com/fsnotify/fsnotify v1.4.9
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v0.0.0-20200528084921-624c01c5539c
	github.com/onsi/ginkgo v1.12.3
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	gopkg.in/intel/multus-cni.v3 v3.4.2
	k8s.io/api v0.18.3
	k8s.io/apimachinery v0.18.3
	k8s.io/client-go v11.0.0+incompatible
)
