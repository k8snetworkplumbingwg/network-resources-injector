package e2e

import (
	"flag"
	"path/filepath"
	"testing"
	"time"

	networkCoreClient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

const (
	defaultPodName      = "nri-e2e-test"
	testNetworkName     = "foo-network"
	pod1stContainerName = "test"
	pod2ndContainerName = "second"
	testNetworkResName  = "example.com/foo"
	interval            = time.Second * 10
	timeout             = time.Second * 30
	minHugepages1Gi     = 2
	minHugepages2Mi     = 1024
)

type ClientSet struct {
	coreclient.CoreV1Interface
}

type NetworkClientSet struct {
	networkCoreClient.K8sCniCncfIoV1Interface
}

var (
	master         *string
	kubeConfigPath *string
	testNs         *string
	cs             *ClientSet
	networkClient  *NetworkClientSet
	kubeConfig     *restclient.Config
)

func init() {
	if home := homedir.HomeDir(); home != "" {
		kubeConfigPath = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "path to your kubeconfig file")
	} else {
		kubeConfigPath = flag.String("kubeconfig", "", "require absolute path to your kubeconfig file")
	}
	master = flag.String("master", "", "Address of Kubernetes API server")
	testNs = flag.String("testnamespace", "default", "namespace for testing")
}

func TestSriovTests(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NRI E2E suite")
}

var _ = BeforeSuite(func(done Done) {
	cfg, err := clientcmd.BuildConfigFromFlags(*master, *kubeConfigPath)
	Expect(err).Should(BeNil())

	kubeConfig = cfg

	cs = &ClientSet{}
	cs.CoreV1Interface = coreclient.NewForConfigOrDie(cfg)

	networkClient = &NetworkClientSet{}
	networkClient.K8sCniCncfIoV1Interface = networkCoreClient.NewForConfigOrDie(cfg)

	close(done)
}, 60)
