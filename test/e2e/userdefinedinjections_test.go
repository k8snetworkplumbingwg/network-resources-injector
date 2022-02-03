package e2e

import (
	"time"

	"github.com/k8snetworkplumbingwg/network-resources-injector/test/util"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Verify 'User Defined Injections'", func() {
	const value = `{
		"user-defined-injections": {
			"customInjection": {
				"op": "add",
				"path": "/metadata/annotations",
				"value": {
						"k8s.v1.cni.cncf.io/networks": "sriov-net-attach-def"
				}
			}
		}
	}
	`

	var pod, pod2 *corev1.Pod
	var configMap *corev1.ConfigMap
	var nad, nad2 *cniv1.NetworkAttachmentDefinition
	var err error

	Context("Positive use cases - expected that NRI will inject correctly user-defined-injections", func() {
		const secondValue = `{
			"user-defined-injections": {
				"secondInjection": {
					"op": "add",
					"path": "/metadata/annotations",
					"value": {
							"top-secret": "password"
					}
				}
			}
		}
		`
		const mergeValueAndSecond = `{
			"user-defined-injections": {
				"customInjection": {
					"op": "add",
					"path": "/metadata/annotations",
					"value": {
							"k8s.v1.cni.cncf.io/networks": "sriov-net-attach-def"
					}
				},
				"secondInjection": {
					"op": "add",
					"path": "/metadata/annotations",
					"value": {
							"top-secret": "password"
					}
				}
			}
		}
		`

		BeforeEach(func() {
			nad = util.GetResourceSelectorOnly(testNetworkName, *testNs, testNetworkResName)
			Expect(util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad, timeout)).Should(BeNil())

			// second network attachment that is used by user user-defined-injections
			nad2 = util.GetResourceSelectorOnly("sriov-net-attach-def", *testNs, "example.com/boo")
			Expect(util.ApplyNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, nad2, timeout)).Should(BeNil())
		})

		AfterEach(func() {
			_ = util.DeletePod(cs.CoreV1Interface, pod, timeout)
			_ = util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, testNetworkName, nad, timeout)
			_ = util.DeleteNetworkAttachmentDefinition(networkClient.K8sCniCncfIoV1Interface, "sriov-net-attach-def", nad2, timeout)

			if nil != configMap {
				_ = util.DeleteConfigMap(cs.CoreV1Interface, configMap, timeout)
				configMap = nil
			}
		})

		It("Config map in correct namespace, one label to inject, POD specification WITHOUT resource name", func() {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", value)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetPodDefinition(*testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("foo-network"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("sriov-net-attach-def"))
		})

		It("Config map in correct namespace, one label to inject, POD specification WITH resource name", func() {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", value)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("foo-network"))
		})

		It("Config map in correct namespace, TWO labels to inject, POD specification WITH resource name", func() {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", mergeValueAndSecond)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "true")
			pod = util.AddMetadataLabel(pod, "secondInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["top-secret"]).Should(ContainSubstring("password"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("foo-network"))
		})

		It("ConfigMap in correct namespace, ONE labels to inject not related to network, POD specification WITH resource name", func() {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", secondValue)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "secondInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["top-secret"]).Should(ContainSubstring("password"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("foo-network"))
		})

		It("Create one POD and next update configMap and create second POD, both should have different injections.", func() {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", value)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("foo-network"))

			// update ConfigMap
			Expect(util.DeleteConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", secondValue)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod2 = util.GetOneNetwork(testNetworkName, *testNs, "default-pod-name")
			pod2 = util.AddMetadataLabel(pod2, "secondInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod2, timeout, interval)).Should(BeNil())
			Expect(pod2.Name).ShouldNot(BeNil())

			defer util.DeletePod(cs.CoreV1Interface, pod2, timeout)

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			pod2, err = util.UpdatePodInfo(cs.CoreV1Interface, pod2, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("foo-network"))

			Expect(pod2.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("sriov-net-attach-def"))
			Expect(pod2.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("foo-network"))
			Expect(pod2.Annotations["top-secret"]).Should(ContainSubstring("password"))
		})

		It("Delete ConfigMap and verify that old label are not removed from existing PODs", func() {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", value)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("foo-network"))

			Expect(util.DeleteConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("foo-network"))
		})

		It("Create POD and valid ConfigMap, next delete ConfigMap and create anther POD, expected without annotations", func() {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", value)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("foo-network"))

			// delete POD and remove map
			Expect(util.DeletePod(cs.CoreV1Interface, pod, timeout)).Should(BeNil())
			Expect(util.DeleteConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("foo-network"))
		})
	})

	Context("Negative use cases - expected that user-defined-injections are not going to be injected", func() {
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

		It("Missing ConfigMap, label is present, expected no modification in POD specification", func() {
			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("foo-network"))
		})

		It("ConfigMap in different namespace than NRI, expected to be ignored", func() {
			configMap = util.GetConfigMap("nri-control-switches", "default")
			configMap = util.AddData(configMap, "config.json", value)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("foo-network"))
		})

		It("ConfigMap in correct namespace, wrong ConfigMap name, expected to be ignored", func() {
			configMap = util.GetConfigMap("nri-user-defined-chaos", "kube-system")
			configMap = util.AddData(configMap, "config.json", value)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("foo-network"))
		})

		It("ConfigMap in correct namespace and name, POD does not define the label", func() {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", value)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("foo-network"))
		})

		It("ConfigMap in correct namespace and name, POD does not define the same label to true as ConfigMap", func() {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", value)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "somethingElseLabel", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("foo-network"))
		})

		It("ConfigMap in correct namespace and name, POD define the same label as ConfigMap but to false", func() {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", value)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "false")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("foo-network"))
		})

		It("ConfigMap in correct namespace and name, POD define the same label as ConfigMap but to some number", func() {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", value)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "15")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("foo-network"))
		})

		It("ConfigMap in correct namespace and name, but different name of label than POD defines in metadata", func() {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", value)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "specificInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).Should(BeNil())
			Expect(pod.Name).ShouldNot(BeNil())

			pod, err = util.UpdatePodInfo(cs.CoreV1Interface, pod, timeout)
			Expect(err).Should(BeNil())

			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).ShouldNot(ContainSubstring("sriov-net-attach-def"))
			Expect(pod.Annotations["k8s.v1.cni.cncf.io/networks"]).Should(ContainSubstring("foo-network"))
		})

		It("ConfigMap in correct namespace and name, labels are correct, missing second NAD, expected POD is not created", func() {
			configMap = util.GetConfigMap("nri-control-switches", "kube-system")
			configMap = util.AddData(configMap, "config.json", value)
			Expect(util.ApplyConfigMap(cs.CoreV1Interface, configMap, timeout)).Should(BeNil())

			// wait for configmap to be consumed by NRI, expected to see in logs something like
			// webhook.go:920] Initializing user-defined injections with key: customInjection, value: {}
			time.Sleep(60 * time.Second)

			pod = util.GetOneNetwork(testNetworkName, *testNs, defaultPodName)
			pod = util.AddMetadataLabel(pod, "customInjection", "true")

			Expect(util.CreateRunningPod(cs.CoreV1Interface, pod, timeout, interval)).ShouldNot(BeNil())
		})
	})
})
