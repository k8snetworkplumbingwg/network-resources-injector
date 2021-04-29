package e2e

import (
	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/k8snetworkplumbingwg/network-resources-injector/test/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Network injection testing", func() {
	var pod *corev1.Pod
	var err error
	var nad *cniv1.NetworkAttachmentDefinition

	Context("one network request", func() {
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
			_ = util.DeletePod(cs.CoreV1Interface, pod, timeout)
			_ = util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)
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

	Context("two network requests", func() {
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
			_ = util.DeletePod(cs.CoreV1Interface, pod, timeout)
			_ = util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)
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
})
