package e2e

//a subset of tests require Hugepages enabled on the test node
import (
	"fmt"
	"strconv"
	"time"

	"github.com/k8snetworkplumbingwg/network-resources-injector/test/util"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func hugepageOrSkip() {
	available, err := util.IsMinHugepagesAvailable(cs.CoreV1Interface, minHugepages1Gi, minHugepages2Mi)
	Expect(err).To(BeNil())
	if !available {
		Skip(fmt.Sprintf("minimum hugepages of %d Gi and %d Mi not found in any k8 worker nodes.",
			minHugepages1Gi, minHugepages2Mi))
	}
}

var _ = Describe("Verify configuration set by control switches", func() {
	var pod *corev1.Pod
	var configMap, userConfigMap *corev1.ConfigMap
	var nad *cniv1.NetworkAttachmentDefinition
	var err error

	BeforeEach(hugepageOrSkip)

	Context("Incorrect nri-control-switches ConfigMap", func() {
		BeforeEach(func() {
			userConfigMap = util.GetConfigMap("nri-user-defined-injections", "kube-system")
			userConfigMap = util.AddData(userConfigMap,
				"customInjection",
				"{\"op\": \"add\", \"path\": \"/metadata/annotations\", \"value\": {\"top-secret\": \"password\"}}")
			err = util.ApplyConfigMap(cs.CoreV1Interface, userConfigMap, timeout)
			Expect(err).Should(BeNil())

			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())
		})

		AfterEach(func() {
			_ = util.DeletePod(cs.CoreV1Interface, pod, timeout)
			_ = util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)

			if nil != userConfigMap {
				_ = util.DeleteConfigMap(cs.CoreV1Interface, userConfigMap, timeout)
				userConfigMap = nil
			}

			if nil != configMap {
				_ = util.DeleteConfigMap(cs.CoreV1Interface, configMap, timeout)
				configMap = nil
			}
		})

		DescribeTable("verify if ConfigMap was accepted", func(configValue string) {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", configValue)
			err = util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)
			Expect(err).Should(BeNil())

			// wait for configmap to be consumed by NRI
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "true")
			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["top-secret"]).Should(ContainSubstring("password"))
		},
			Entry("without features definition, correct namespace", ""),
			Entry("all features disabled, correct namespace", `{
				"features": {
					"enableHugePageDownApi": false,
					"enableHonorExistingResources": false,
					"enableCustomizedInjection": false,
					"enableResourceName": false
				}
			}
			`),
			Entry("unknown features or misspelled, correct namespace", `{
				"features": {
					"someSuperFeature": false,
					"resourceName": false
				}
			}
			`),
		)
	})
})

var _ = Describe("Verify configuration set by control switches", func() {
	var pod *corev1.Pod
	var configMap *corev1.ConfigMap
	var nad *cniv1.NetworkAttachmentDefinition
	var err error
	var stdoutString, stderrString string

	BeforeEach(hugepageOrSkip)

	Context("Check downward API setting with huge pages", func() {
		BeforeEach(func() {
			stdoutString = ""
			stderrString = ""
		})

		AfterEach(func() {
			_ = util.DeletePod(cs.CoreV1Interface, pod, timeout)
			_ = util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)

			if nil != configMap {
				_ = util.DeleteConfigMap(cs.CoreV1Interface, configMap, timeout)
				configMap = nil
			}
		})

		It("POD with annotation about resourceName, hugepages1Gi limit and memory size are defined and are equal", func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddToPodDefinitionHugePages1Gi(pod, 2, 2, 0)
			pod = util.AddToPodDefinitionMemory(pod, 2, 2, 0)

			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			// Check new environment variable
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "printenv")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("HOSTNAME=" + pod.Name))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "ls /etc/podnetinfo")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_limit_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_request_" + pod1stContainerName))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_1G_limit_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err := strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(2048)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_1G_request_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(2048)))

			err = util.DeletePod(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			const disabledState = `{
				"features": {
					"enableHugePageDownApi": false,
					"enableHonorExistingResources": false,
					"enableCustomizedInjection": false,
					"enableResourceName": false
				}
			}
			`

			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", disabledState)
			err = util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)
			Expect(err).Should(BeNil())

			// wait for configmap to be consumed by NRI
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddToPodDefinitionHugePages1Gi(pod, 2, 2, 0)
			pod = util.AddToPodDefinitionMemory(pod, 2, 2, 0)

			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			// Check new environment variable
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "printenv")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("HOSTNAME=" + pod.Name))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "ls /etc/podnetinfo")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_limit_" + pod1stContainerName))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_request_" + pod1stContainerName))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_1G_limit_"+pod1stContainerName)
			Expect(err).ShouldNot(BeNil())

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_1G_request_"+pod1stContainerName)
			Expect(err).ShouldNot(BeNil())
		})

	})
})
