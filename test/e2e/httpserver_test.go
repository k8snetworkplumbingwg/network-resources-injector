package e2e

import (
	"github.com/k8snetworkplumbingwg/network-resources-injector/test/util"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Network injection testing", func() {
	var pod *corev1.Pod
	var err error

	Context("one network request", func() {
		BeforeEach(func() {
			pod = util.GetOneNetwork(testNetworkName, *testNs)
			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())
			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())
		})

		AfterEach(func() {
			util.DeletePod(cs.CoreV1Interface, pod, timeout)
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
			pod = util.GetMultiNetworks([]string{testNetworkName, testNetworkName}, *testNs)
			err = util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)
			Expect(err).Should(BeNil())
			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
		})

		AfterEach(func() {
			util.DeletePod(cs.CoreV1Interface, pod, timeout)
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
