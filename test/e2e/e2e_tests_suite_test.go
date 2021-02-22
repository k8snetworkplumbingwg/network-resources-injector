package e2e

import (
	"flag"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)


const (
	testNetworkName    = "foo-network"
	testNetworkResName = "example.com/foo"
	interval           = time.Second * 10
	timeout            = time.Second * 30
)

var (
	master         *string
	kubeConfigPath *string
	testNs         *string
	cs             *ClientSet
)

type ClientSet struct {
	coreclient.CoreV1Interface
}

func init()  {
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

	cs = &ClientSet{}
	cs.CoreV1Interface = coreclient.NewForConfigOrDie(cfg)

	close(done)
}, 60)
