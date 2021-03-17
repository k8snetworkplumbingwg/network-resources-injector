package e2e

import (
	"github.com/k8snetworkplumbingwg/network-resources-injector/test/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Verify that resource and POD which consumes resource cannot be in different namespaces", func() {
	var pod *corev1.Pod
	var nad *cniv1.NetworkAttachmentDefinition
	var err error

	Context("network attachment definition configuration error", func() {
		It("Missing network attachment definition, try to setup POD in default namespace", func() {
			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("could not get Network Attachment Definition default/foo-network"))
		})

		It("Correct network name in CRD, but the namespace if different than in POD specification", func() {
			testNamespace := "mysterious"
			err = util.CreateNamespace(cs.CoreV1Interface, testNamespace, timeout)
			Expect(err).Should(BeNil())

			nad = util.GetResourceSelectorOnly(testNetworkName, testNamespace, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("could not get Network Attachment Definition default/foo-network"))

			err = util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)
			Expect(err).Should(BeNil())

			err = util.DeleteNamespace(cs.CoreV1Interface, testNamespace, timeout)
			Expect(err).Should(BeNil())
		})

		It("CRD in default namespace, and POD in custom namespace", func() {
			testNamespace := "mysterious"
			err = util.CreateNamespace(cs.CoreV1Interface, testNamespace, timeout)
			Expect(err).Should(BeNil())

			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, testNamespace, defaultPodName)
			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).ShouldNot(BeNil())
			Expect(err.Error()).Should(ContainSubstring("could not get Network Attachment Definition mysterious/foo-network"))

			err = util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)
			Expect(err).Should(BeNil())

			err = util.DeleteNamespace(cs.CoreV1Interface, testNamespace, timeout)
			Expect(err).Should(BeNil())
		})
	})
})

var _ = Describe("Network injection testing", func() {
	var pod *corev1.Pod
	var nad *cniv1.NetworkAttachmentDefinition
	var err error

	Context("one network request in default namespace", func() {
		BeforeEach(func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())
			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())
		})

		AfterEach(func() {
			util.DeletePod(cs.CoreV1Interface, pod, timeout)
			err = util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)
			Expect(err).Should(BeNil())
		})

		It("should have one limit injected", func() {
			limNo, ok := pod.Spec.Containers[0].Resources.Limits[testNetworkResName]
			Expect(ok).Should(BeTrue())
			Expect(limNo.String()).Should(Equal("1"))
		})

		It("should have one request injected", func() {
			limNo, ok := pod.Spec.Containers[0].Resources.Requests[testNetworkResName]
			Expect(ok).Should(BeTrue())
			Expect(limNo.String()).Should(Equal("1"))
		})
	})

	Context("two network requests in default namespace", func() {
		BeforeEach(func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetMultiNetworks([]string{testNetworkName, testNetworkName}, *testNs, defaultPodName)
			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
		})

		AfterEach(func() {
			util.DeletePod(cs.CoreV1Interface, pod, timeout)
			util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)
		})

		It("should have two limits injected", func() {
			limNo, ok := pod.Spec.Containers[0].Resources.Limits[testNetworkResName]
			Expect(ok).Should(BeTrue())
			Expect(limNo.String()).Should(Equal("2"))
		})

		It("should have two requests injected", func() {
			limNo, ok := pod.Spec.Containers[0].Resources.Requests[testNetworkResName]
			Expect(ok).Should(BeTrue())
			Expect(limNo.String()).Should(Equal("2"))
		})
	})

	Context("one network request in custom namespace", func() {
		BeforeEach(func() {
			testNamespace := "mysterious"
			err = util.CreateNamespace(cs.CoreV1Interface, testNamespace, timeout)
			Expect(err).Should(BeNil())

			nad = util.GetResourceSelectorOnly(testNetworkName, testNamespace, testNetworkResName)
			err = util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)
			Expect(err).Should(BeNil())

			pod = util.GetOneNetwork(testNetworkName, testNamespace, defaultPodName)
			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())
			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())
		})

		AfterEach(func() {
			testNamespace := "mysterious"
			err = util.DeleteNamespace(cs.CoreV1Interface, testNamespace, timeout)
			Expect(err).Should(BeNil())
		})

		It("should have one limit injected", func() {
			limNo, ok := pod.Spec.Containers[0].Resources.Limits[testNetworkResName]
			Expect(ok).Should(BeTrue())
			Expect(limNo.String()).Should(Equal("1"))
		})

		It("should have one request injected", func() {
			limNo, ok := pod.Spec.Containers[0].Resources.Requests[testNetworkResName]
			Expect(ok).Should(BeTrue())
			Expect(limNo.String()).Should(Equal("1"))
		})
	})
})
