package e2e

//a subset of tests require Hugepages enabled on the test node
import (
	"fmt"
	"strconv"

	"github.com/k8snetworkplumbingwg/network-resources-injector/test/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	corev1 "k8s.io/api/core/v1"
)

func hugepageOrSkip() {
	available, err := util.IsMinHugepagesAvailable(cs.CoreV1Interface, minHugepages1Gi, minHugepages2Mi)
	Expect(err).To(BeNil())
	if !available {
		Skip(fmt.Sprintf("minimum hugepages of %d Gi and %d Mi not found in any k8 workner nodes.",
			minHugepages1Gi, minHugepages2Mi))
	}
}

var _ = Describe("POD in default namespace with downwarAPI already defined", func() {
	var pod *corev1.Pod
	var nad *cniv1.NetworkAttachmentDefinition
	var err error
	var stdoutString, stderrString string

	BeforeEach(hugepageOrSkip)

	Context("POD with resource name requests 1Gi hugepages, add namespace information via DownwardAPI", func() {
		BeforeEach(func() {
			stdoutString = ""
			stderrString = ""

			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddToPodDefinitionHugePages1Gi(pod, 2, 2, 0)
			pod = util.AddToPodDefinitionMemory(pod, 2, 2, 0)
		})

		AfterEach(func() {
			util.DeletePod(cs.CoreV1Interface, pod, timeout)
			util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)
		})

		It("POD have downwardAPI defined to /etc/podnetinfo, so the same name and folder as NRI is using", func() {
			pod = util.AddToPodDefinitionVolumesWithDownwardAPI(pod, "/etc/podnetinfo", "podnetinfo", 0)

			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("Duplicate value: \"podnetinfo\""))

			// This test fails by puprose, it should be possible to have podnetinfo defined already in POD spec before NRI will inject resources
			// https://github.com/k8snetworkplumbingwg/network-resources-injector/issues/43
			Expect(err).Should(BeNil())
		})

		It("POD have downwardAPI defined to /etc/podnetinfo, but volumeName is different than the one used by NRI", func() {
			pod = util.AddToPodDefinitionVolumesWithDownwardAPI(pod, "/etc/podnetinfo", "something-else", 0)

			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("Invalid value: \"/etc/podnetinfo\": must be unique"))

			// This test fails by puprose, it should be possible to have podnetinfo defined already in POD spec before NRI will inject resources
			// https://github.com/k8snetworkplumbingwg/network-resources-injector/issues/43
			Expect(err).Should(BeNil())
		})

		It("POD have downwardAPI defined to /etc/somethingElse, but volumeName is the same as NRI is using", func() {
			pod = util.AddToPodDefinitionVolumesWithDownwardAPI(pod, "/etc/somethingElse", "podnetinfo", 0)

			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("Duplicate value: \"podnetinfo\""))

			// This test fails by puprose, it should be possible to have podnetinfo defined already in POD spec before NRI will inject resources
			// https://github.com/k8snetworkplumbingwg/network-resources-injector/issues/43
			Expect(err).Should(BeNil())
		})

		It("POD have downwardAPI defined to /etc/somethingElse and other volumeName that the NRI is using", func() {
			pod = util.AddToPodDefinitionVolumesWithDownwardAPI(pod, "/etc/somethingElse", "something-else", 0)

			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "ls /etc/podnetinfo")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_limit_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_request_" + pod1stContainerName))
			Expect(stdoutString).ShouldNot(ContainSubstring("namespace"))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "ls /etc/somethingElse")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_limit_" + pod1stContainerName))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_request_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("namespace"))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/somethingElse/namespace")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(Equal("default"))
		})
	})
})

var _ = Describe("Expose hugepages via Downward API, POD in default namespace", func() {
	var pod *corev1.Pod
	var nad *cniv1.NetworkAttachmentDefinition
	var err error
	var stdoutString, stderrString string

	BeforeEach(hugepageOrSkip)

	Context("Check in environment where it is possible to create hugepages", func() {
		BeforeEach(func() {
			stdoutString = ""
			stderrString = ""
		})

		AfterEach(func() {
			util.DeletePod(cs.CoreV1Interface, pod, timeout)
			util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)
		})

		It("POD without annotation about resourceName, hugepages limit and memory size are defined and are equal", func() {
			nad = util.GetWithoutAnnotations(testNetworkName, *testNs)
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

			// NRI will not provide Downward API for huge pages
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "ls /etc/podnetinfo")
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(Equal("command terminated with exit code 1"))
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(Equal(""))
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
		})

		It("POD with annotation about resourceName, hugepages 2Mi limit and memory size are defined and are equal", func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddToPodDefinitionHugePages2Mi(pod, 1000, 1000, 0)
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
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_limit_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_request_" + pod1stContainerName))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_2M_limit_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err := strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1000)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_2M_request_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1000)))
		})

		It("POD with annotation about resourceName, hugepages 1Gi limit and cpu request are defined", func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddToPodDefinitionHugePages1Gi(pod, 2, 2, 0)
			pod = util.AddToPodDefinitionCpuLimits(pod, 4, 0)

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
		})
	})

	Context("Mix different hugepages and verify if files content is correct", func() {
		BeforeEach(func() {
			stdoutString = ""
			stderrString = ""
		})

		AfterEach(func() {
			util.DeletePod(cs.CoreV1Interface, pod, timeout)
			util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)
		})

		It("Same amounts in request and limits, use 1Gi and 2Mi hugepages", func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddToPodDefinitionHugePages1Gi(pod, 2, 2, 0)
			pod = util.AddToPodDefinitionHugePages2Mi(pod, 1000, 1000, 0)
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
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_limit_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_request_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_limit_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_request_" + pod1stContainerName))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_2M_limit_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err := strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1000)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_2M_request_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1000)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_1G_limit_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(2048)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_1G_request_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(2048)))
		})

		It("Different amounts in request and limits, use 1Gi and 2Mi hugepages", func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddToPodDefinitionHugePages1Gi(pod, 1, 1, 0)
			pod = util.AddToPodDefinitionHugePages2Mi(pod, 250, 250, 0)
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
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_limit_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_request_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_limit_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_request_" + pod1stContainerName))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_2M_limit_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err := strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(250)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_2M_request_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(250)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_1G_limit_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1024)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_1G_request_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1024)))
		})
	})

	Context("Mix different hugepages and verify if files content and names are correct when there are more containers in POD spec", func() {
		BeforeEach(func() {
			stdoutString = ""
			stderrString = ""
		})

		AfterEach(func() {
			util.DeletePod(cs.CoreV1Interface, pod, timeout)
			util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)
		})

		It("Same amounts in request and limits, use 1Gi and 2Mi hugepages, only on first container", func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetworkTwoContainers(testNetworkName, *testNs, defaultPodName, pod2ndContainerName)
			pod = util.AddToPodDefinitionHugePages1Gi(pod, 2, 2, 0)
			pod = util.AddToPodDefinitionHugePages2Mi(pod, 1000, 1000, 0)
			pod = util.AddToPodDefinitionMemory(pod, 2, 2, 0)

			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			// Check new environment variable
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "printenv")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("HOSTNAME=" + pod.Name))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "printenv")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("HOSTNAME=" + pod.Name))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "ls /etc/podnetinfo")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_limit_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_request_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_limit_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_request_" + pod1stContainerName))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_2M_limit_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err := strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1000)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_2M_request_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1000)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_1G_limit_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(2048)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_1G_request_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(2048)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "ls /etc/podnetinfo")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_2M_limit_" + pod2ndContainerName))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_2M_request_" + pod2ndContainerName))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_limit_" + pod2ndContainerName))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_request_" + pod2ndContainerName))
		})

		It("Same amounts in request and limits, use 1Gi and 2Mi hugepages, only on second container", func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetworkTwoContainers(testNetworkName, *testNs, defaultPodName, pod2ndContainerName)
			pod = util.AddToPodDefinitionHugePages1Gi(pod, 2, 2, 1)
			pod = util.AddToPodDefinitionHugePages2Mi(pod, 1000, 1000, 1)
			pod = util.AddToPodDefinitionMemory(pod, 2, 2, 1)

			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			// Check new environment variable
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "printenv")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("HOSTNAME=" + pod.Name))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "printenv")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("HOSTNAME=" + pod.Name))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "ls /etc/podnetinfo")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_2M_limit_" + pod1stContainerName))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_2M_request_" + pod1stContainerName))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_limit_" + pod1stContainerName))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_request_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_limit_" + pod2ndContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_request_" + pod2ndContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_limit_" + pod2ndContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_request_" + pod2ndContainerName))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "ls /etc/podnetinfo")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_2M_limit_" + pod1stContainerName))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_2M_request_" + pod1stContainerName))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_limit_" + pod1stContainerName))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_request_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_limit_" + pod2ndContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_request_" + pod2ndContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_limit_" + pod2ndContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_request_" + pod2ndContainerName))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "cat /etc/podnetinfo/hugepages_2M_limit_"+pod2ndContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err := strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1000)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "cat /etc/podnetinfo/hugepages_2M_request_"+pod2ndContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1000)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "cat /etc/podnetinfo/hugepages_1G_limit_"+pod2ndContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(2048)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "cat /etc/podnetinfo/hugepages_1G_request_"+pod2ndContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(2048)))
		})

		It("Same amounts in request and limits, use 1Gi and 2Mi hugepages, on both container", func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetworkTwoContainers(testNetworkName, *testNs, defaultPodName, pod2ndContainerName)
			// add hugepages to first container
			pod = util.AddToPodDefinitionHugePages1Gi(pod, 2, 2, 0)
			pod = util.AddToPodDefinitionHugePages2Mi(pod, 1000, 1000, 0)
			pod = util.AddToPodDefinitionMemory(pod, 2, 2, 0)
			// add hugepages to second container
			pod = util.AddToPodDefinitionHugePages1Gi(pod, 2, 2, 1)
			pod = util.AddToPodDefinitionHugePages2Mi(pod, 1000, 1000, 1)
			pod = util.AddToPodDefinitionMemory(pod, 2, 2, 1)

			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			// Check new environment variable
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "printenv")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("HOSTNAME=" + pod.Name))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "printenv")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("HOSTNAME=" + pod.Name))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "ls /etc/podnetinfo")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_limit_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_request_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_limit_" + pod1stContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_request_" + pod1stContainerName))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_2M_limit_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err := strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1000)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_2M_request_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1000)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_1G_limit_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(2048)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "cat /etc/podnetinfo/hugepages_1G_request_"+pod1stContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(2048)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "ls /etc/podnetinfo")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_limit_" + pod2ndContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_2M_request_" + pod2ndContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_limit_" + pod2ndContainerName))
			Expect(stdoutString).Should(ContainSubstring("hugepages_1G_request_" + pod2ndContainerName))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "cat /etc/podnetinfo/hugepages_2M_limit_"+pod2ndContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1000)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "cat /etc/podnetinfo/hugepages_2M_request_"+pod2ndContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(1000)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "cat /etc/podnetinfo/hugepages_1G_limit_"+pod2ndContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(2048)))

			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod2ndContainerName, "cat /etc/podnetinfo/hugepages_1G_request_"+pod2ndContainerName)
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(Equal(""))
			size, err = strconv.ParseInt(stdoutString, 10, 32)
			Expect(err).Should(BeNil())
			Expect(size).Should(Equal(int64(2048)))
		})
	})
})

// doesn't need to have hugepages enabled on the host for test to execute successfully
var _ = Describe("Expose hugepages via Downward API, POD in default namespace", func() {
	var pod *corev1.Pod
	var nad *cniv1.NetworkAttachmentDefinition
	var err error
	var stdoutString, stderrString string

	Context("request 0 pages", func() {
		BeforeEach(func() {
			stdoutString = ""
			stderrString = ""
		})

		AfterEach(func() {
			util.DeletePod(cs.CoreV1Interface, pod, timeout)
			util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)
		})

		It("POD without annotation about resourceName, hugepages limit and memory size are defined and are equal", func() {
			nad = util.GetWithoutAnnotations(testNetworkName, *testNs)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddToPodDefinitionHugePages1Gi(pod, 0, 0, 0)
			pod = util.AddToPodDefinitionMemory(pod, 0, 0, 0)

			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			// Check new environment variable
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "printenv")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("HOSTNAME=" + pod.Name))

			// NRI will not provide Downward API for huge pages
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "ls /etc/podnetinfo")

			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(Equal("command terminated with exit code 1"))
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(Equal(""))
		})

		It("POD with annotation about resourceName, hugepages limit and memory size are defined and are equal", func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddToPodDefinitionHugePages1Gi(pod, 0, 0, 0)
			pod = util.AddToPodDefinitionMemory(pod, 0, 0, 0)

			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			// Check new environment variable
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "printenv")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("HOSTNAME=" + pod.Name))

			// NRI will not provide Downward API for huge pages
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "ls /etc/podnetinfo")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_limit_test"))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_request_test"))
		})

		It("POD with annotation about resourceName, hugepages limit and memory size are defined and are equal", func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddToPodDefinitionHugePages2Mi(pod, 0, 0, 0)
			pod = util.AddToPodDefinitionMemory(pod, 0, 0, 0)

			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			// Check new environment variable
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "printenv")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("HOSTNAME=" + pod.Name))

			// NRI will not provide Downward API for huge pages
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "ls /etc/podnetinfo")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_2G_limit_test"))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_2G_request_test"))
		})

		It("POD with annotation about resourceName, hugepages limit and cpu request are defined", func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddToPodDefinitionHugePages1Gi(pod, 0, 0, 0)
			pod = util.AddToPodDefinitionCpuLimits(pod, 1, 0)

			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			// Check new environment variable
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "printenv")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).Should(ContainSubstring("HOSTNAME=" + pod.Name))

			// NRI will not provide Downward API for huge pages
			stdoutString, stderrString, err = util.ExecuteCommand(cs.CoreV1Interface, kubeConfig, pod.Name, *testNs, pod1stContainerName, "ls /etc/podnetinfo")
			Expect(err).Should(BeNil())
			Expect(stderrString).Should(Equal(""))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_limit_test"))
			Expect(stdoutString).ShouldNot(ContainSubstring("hugepages_1G_request_test"))
		})
	})
})
