package e2e

//a subset of tests require Hugepages enabled on the test node
import (
	"fmt"
	"time"

	"github.com/k8snetworkplumbingwg/network-resources-injector/test/util"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Verify configuration set by control switches", func() {
	var pod *corev1.Pod
	var configMap *corev1.ConfigMap
	var nad *cniv1.NetworkAttachmentDefinition
	var err error

	BeforeEach(hugepageOrSkip)

	Context("Incorrect nri-control-switches ConfigMap", func() {
		BeforeEach(func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			Expect(util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)).Should(BeNil())
		})

		AfterEach(func() {
			_ = util.DeletePod(cs.CoreV1Interface, pod, timeout)
			_ = util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)

			if nil != configMap {
				_ = util.DeleteConfigMap(cs.CoreV1Interface, configMap, timeout)
				configMap = nil
			}
		})

		const secondValue = `
			"user-defined-injections": {
				"customInjection": {
					"op": "add",
					"path": "/metadata/annotations",
					"value": {
							"top-secret": "password"
					}
				}
			},
		`

		DescribeTable("verify if ConfigMap was accepted", func(configValue string) {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", "{"+secondValue+configValue+"}")
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "true")
			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			fmt.Println(pod)
			Expect(err).Should(BeNil())
			Expect(pod.Annotations["top-secret"]).Should(ContainSubstring("password"))
		},
			Entry("without features definition, correct namespace", `
				"networkResourceNameKeys": ["k8s.v1.cni.cncf.io/resourceName", "k8s.v1.cni.cncf.io/bridgeName"]
			`),
			Entry("all features disabled, correct namespace", `
				"features": {
					"enableHugePageDownApi": false,
					"enableHonorExistingResources": false,
					"enableCustomizedInjection": false,
					"enableResourceName": false
				}
			`),
			Entry("unknown features or misspelled, correct namespace", `
				"features": {
					"someSuperFeature": false,
					"resourceName": false
				}
			`),
		)
	})
})
