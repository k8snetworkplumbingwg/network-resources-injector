// Copyright (c) 2017 Intel Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package webhook

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"gopkg.in/k8snetworkplumbingwg/multus-cni.v4/pkg/types"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k8snetworkplumbingwg/network-resources-injector/pkg/controlswitches"
	nritypes "github.com/k8snetworkplumbingwg/network-resources-injector/pkg/types"
)

func createBool(value bool) *bool {
	return &value
}

func createString(value string) *string {
	return &value
}

func deserializeNetworkAttachmentDefinition(ar *admissionv1.AdmissionReview) (cniv1.NetworkAttachmentDefinition, error) {
	/* unmarshal NetworkAttachmentDefinition from AdmissionReview request */
	netAttachDef := cniv1.NetworkAttachmentDefinition{}
	err := json.Unmarshal(ar.Request.Object.Raw, &netAttachDef)
	return netAttachDef, err
}

var _ = Describe("Webhook", func() {
	Describe("Preparing Admission Review Response", func() {
		Context("Admission Review Request is nil", func() {
			It("should return error", func() {
				ar := &admissionv1.AdmissionReview{}
				ar.Request = nil
				Expect(prepareAdmissionReviewResponse(false, "", ar)).To(HaveOccurred())
			})
		})
		Context("Message is not empty", func() {
			It("should set message in the response", func() {
				ar := &admissionv1.AdmissionReview{}
				ar.Request = &admissionv1.AdmissionRequest{
					UID: "fake-uid",
				}
				err := prepareAdmissionReviewResponse(false, "some message", ar)
				Expect(err).NotTo(HaveOccurred())
				Expect(ar.Response.Result.Message).To(Equal("some message"))
			})
		})
	})

	Describe("Deserializing Admission Review", func() {
		Context("It's not an Admission Review", func() {
			It("should return an error", func() {
				body := []byte("some-invalid-body")
				_, err := deserializeAdmissionReview(body)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Deserializing Network Attachment Definition", func() {
		Context("It's not an Network Attachment Definition", func() {
			It("should return an error", func() {
				ar := &admissionv1.AdmissionReview{}
				ar.Request = &admissionv1.AdmissionRequest{}
				_, err := deserializeNetworkAttachmentDefinition(ar)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Deserializing Pod", func() {
		Context("It's not a Pod", func() {
			It("should return an error", func() {
				ar := &admissionv1.AdmissionReview{}
				ar.Request = &admissionv1.AdmissionRequest{}
				_, err := deserializePod(ar)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Writing a response", func() {
		Context("with an AdmissionReview", func() {
			It("should be marshaled and written to a HTTP Response Writer", func() {
				w := httptest.NewRecorder()
				ar := &admissionv1.AdmissionReview{}
				ar.Response = &admissionv1.AdmissionResponse{
					UID:     "fake-uid",
					Allowed: true,
					Result: &metav1.Status{
						Message: "fake-msg",
					},
				}
				expected := []byte(`{"response":{"uid":"fake-uid","allowed":true,"status":{"metadata":{},"message":"fake-msg"}}}`)
				writeResponse(w, ar)
				Expect(w.Body.Bytes()).To(Equal(expected))
			})
		})
	})

	Describe("Handling requests", func() {
		BeforeEach(func() {
			structure := controlswitches.SetupControlSwitchesUnitTests(createBool(false), createBool(false), createString(""))
			structure.InitControlSwitches()
			SetControlSwitches(structure)
		})

		Context("Request body is empty", func() {
			It("mutate - should return an error", func() {
				req := httptest.NewRequest("POST", "https://fakewebhook/mutate", nil)
				w := httptest.NewRecorder()
				MutateHandler(w, req)
				resp := w.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("Content type is not application/json", func() {
			It("mutate - should return an error", func() {
				req := httptest.NewRequest("POST", "https://fakewebhook/mutate", bytes.NewBufferString("fake-body"))
				req.Header.Set("Content-Type", "invalid-type")
				w := httptest.NewRecorder()
				MutateHandler(w, req)
				resp := w.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusUnsupportedMediaType))
			})
		})
	})

	Describe("Dynamic Hugepages Detection", func() {
		DescribeTable("Hugepage resource name parsing",
			func(resourceName string, expectedSize string, shouldMatch bool) {
				matches := HugepageRegex.FindStringSubmatch(resourceName)
				if shouldMatch {
					Expect(matches).NotTo(BeNil())
					Expect(len(matches)).To(Equal(2))
					Expect(matches[1]).To(Equal(expectedSize))
				} else {
					Expect(matches).To(BeNil())
				}
			},
			Entry("1Gi hugepages", "hugepages-1Gi", "1Gi", true),
			Entry("2Mi hugepages", "hugepages-2Mi", "2Mi", true),
			Entry("1Mi hugepages", "hugepages-1Mi", "1Mi", true),
			Entry("512Ki hugepages", "hugepages-512Ki", "512Ki", true),
			Entry("4Gi hugepages", "hugepages-4Gi", "4Gi", true),
			Entry("non-hugepage resource", "intel.com/sriov", "", false),
			Entry("malformed hugepage", "hugepage-1Gi", "", false),
			Entry("empty string", "", "", false),
		)
	})

	Describe("Hugepage Detection Logic", func() {
		Context("Container with hugepage requests", func() {
			It("should detect 1Gi hugepage requests", func() {
				container := corev1.Container{
					Name: "test-container",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"hugepages-1Gi": resource.MustParse("2Gi"),
						},
					},
				}
				containers := []corev1.Container{container}
				initialPatch := []nritypes.JSONPatchOperation{}

				updatedPatch, hugepageResourceList := processHugepagesForDownwardAPI(initialPatch, containers)

				Expect(len(hugepageResourceList)).To(Equal(1))
				Expect(hugepageResourceList[0].ResourceName).To(Equal("requests.hugepages-1Gi"))
				Expect(hugepageResourceList[0].ContainerName).To(Equal("test-container"))
				Expect(hugepageResourceList[0].Path).To(Equal("hugepages_1G_request_test-container"))
				// Verify that environment variable patch was added
				Expect(len(updatedPatch)).To(BeNumerically(">", 0))
			})

			It("should detect multiple hugepage sizes", func() {
				container := corev1.Container{
					Name: "multi-hugepage-container",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"hugepages-1Gi":   resource.MustParse("2Gi"),
							"hugepages-2Mi":   resource.MustParse("1024Mi"),
							"hugepages-1Mi":   resource.MustParse("512Mi"),
							"hugepages-512Ki": resource.MustParse("256Mi"),
						},
					},
				}

				containers := []corev1.Container{container}
				initialPatch := []nritypes.JSONPatchOperation{}

				_, hugepageResourceList := processHugepagesForDownwardAPI(initialPatch, containers)
				Expect(len(hugepageResourceList)).To(Equal(4))

				// Verify all hugepage sizes are detected
				resourceNames := make([]string, len(hugepageResourceList))
				for i, hp := range hugepageResourceList {
					resourceNames[i] = hp.ResourceName
				}
				Expect(resourceNames).To(ContainElements(
					"requests.hugepages-1Gi",
					"requests.hugepages-2Mi",
					"requests.hugepages-1Mi",
					"requests.hugepages-512Ki",
				))
			})

			It("should ignore zero-valued hugepage requests", func() {
				container := corev1.Container{
					Name: "zero-hugepage-container",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"hugepages-1Gi": resource.MustParse("0"),
						},
					},
				}

				containers := []corev1.Container{container}
				initialPatch := []nritypes.JSONPatchOperation{}

				_, hugepageResourceList := processHugepagesForDownwardAPI(initialPatch, containers)
				Expect(len(hugepageResourceList)).To(Equal(0))
			})
		})

		Context("Container with hugepage limits", func() {
			It("should detect hugepage limits", func() {
				container := corev1.Container{
					Name: "limit-container",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							"hugepages-2Mi": resource.MustParse("1Gi"),
							"hugepages-4Gi": resource.MustParse("8Gi"),
						},
					},
				}

				containers := []corev1.Container{container}
				initialPatch := []nritypes.JSONPatchOperation{}

				_, hugepageResourceList := processHugepagesForDownwardAPI(initialPatch, containers)
				Expect(len(hugepageResourceList)).To(Equal(2))

				// Check both limits are detected
				resourceNames := make([]string, len(hugepageResourceList))
				paths := make([]string, len(hugepageResourceList))
				for i, hp := range hugepageResourceList {
					resourceNames[i] = hp.ResourceName
					paths[i] = hp.Path
				}
				Expect(resourceNames).To(ContainElements(
					"limits.hugepages-2Mi",
					"limits.hugepages-4Gi",
				))
				Expect(paths).To(ContainElements(
					"hugepages_2M_limit_limit-container",
					"hugepages_4G_limit_limit-container",
				))
			})
		})

		Context("Container with both requests and limits", func() {
			It("should detect both requests and limits", func() {
				container := corev1.Container{
					Name: "both-container",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"hugepages-1Gi": resource.MustParse("2Gi"),
						},
						Limits: corev1.ResourceList{
							"hugepages-1Gi": resource.MustParse("2Gi"),
							"hugepages-2Mi": resource.MustParse("1Gi"),
						},
					},
				}

				containers := []corev1.Container{container}
				initialPatch := []nritypes.JSONPatchOperation{}

				_, hugepageResourceList := processHugepagesForDownwardAPI(initialPatch, containers)
				Expect(len(hugepageResourceList)).To(Equal(3))

				// Verify all are detected
				resourceNames := make([]string, len(hugepageResourceList))
				for i, hp := range hugepageResourceList {
					resourceNames[i] = hp.ResourceName
				}
				Expect(resourceNames).To(ContainElements(
					"requests.hugepages-1Gi",
					"limits.hugepages-1Gi",
					"limits.hugepages-2Mi",
				))
			})
		})

		Context("Container with non-hugepage resources", func() {
			It("should ignore non-hugepage resources", func() {
				container := corev1.Container{
					Name: "non-hugepage-container",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							"intel.com/sriov": resource.MustParse("1"),
							"memory":          resource.MustParse("1Gi"),
							"cpu":             resource.MustParse("500m"),
							"nvidia.com/gpu":  resource.MustParse("1"),
						},
					},
				}

				containers := []corev1.Container{container}
				initialPatch := []nritypes.JSONPatchOperation{}

				_, hugepageResourceList := processHugepagesForDownwardAPI(initialPatch, containers)
				Expect(len(hugepageResourceList)).To(Equal(0))
			})
		})
	})

	DescribeTable("Get network selections",

		func(annotateKey string, pod corev1.Pod, patchs []nritypes.JSONPatchOperation, out string, shouldExist bool) {
			nets, exist := getNetworkSelections(annotateKey, pod, patchs)
			Expect(exist).To(Equal(shouldExist))
			Expect(nets).Should(Equal(out))
		},
		Entry(
			"get from pod original annotation",
			"k8s.v1.cni.cncf.io/networks",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: map[string]string{"k8s.v1.cni.cncf.io/networks": "sriov-net"},
				},
				Spec: corev1.PodSpec{},
			},
			[]nritypes.JSONPatchOperation{
				{
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net-user-defined"},
				},
			},
			"sriov-net",
			true,
		),
		Entry(
			"get from pod user-defined annotation",
			"k8s.v1.cni.cncf.io/networks",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: map[string]string{"v1.multus-cni.io/default-network": "sriov-net"},
				},
				Spec: corev1.PodSpec{},
			},
			[]nritypes.JSONPatchOperation{
				{
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net-user-defined"},
				},
			},
			"sriov-net-user-defined",
			true,
		),
		Entry(
			"get from pod user-defined annotation",
			"k8s.v1.cni.cncf.io/networks",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test",
					Annotations: map[string]string{"v1.multus-cni.io/default-network": "sriov-net"},
				},
				Spec: corev1.PodSpec{},
			},
			[]nritypes.JSONPatchOperation{
				{
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"v1.multus-cni.io/default-network": "sriov-net-user-defined"},
				},
			},
			"",
			false,
		),
	)

	var emptyList []*types.NetworkSelectionElement
	DescribeTable("Network selection elements parsing",

		func(in string, out []*types.NetworkSelectionElement, shouldFail bool) {
			actualOut, err := parsePodNetworkSelections(in, "default")
			Expect(actualOut).To(ConsistOf(out))
			if shouldFail {
				Expect(err).To(HaveOccurred())
			}
		},
		Entry(
			"empty config",
			"",
			emptyList,
			false,
		),
		Entry(
			"csv - correct ns/net@if format",
			"ns1/net1@eth0",
			[]*types.NetworkSelectionElement{
				{
					Namespace:        "ns1",
					Name:             "net1",
					InterfaceRequest: "eth0",
				},
			},
			false,
		),
		Entry(
			"csv - correct net@if format",
			"net1@eth0",
			[]*types.NetworkSelectionElement{
				{
					Namespace:        "default",
					Name:             "net1",
					InterfaceRequest: "eth0",
				},
			},
			false,
		),
		Entry(
			"csv - correct *name-only* format",
			"net1",
			[]*types.NetworkSelectionElement{
				{
					Namespace:        "default",
					Name:             "net1",
					InterfaceRequest: "",
				},
			},
			false,
		),
		Entry(
			"csv - correct ns/net format",
			"ns1/net1",
			[]*types.NetworkSelectionElement{
				{
					Namespace:        "ns1",
					Name:             "net1",
					InterfaceRequest: "",
				},
			},
			false,
		),
		Entry(
			"csv - correct multiple networks format",
			"ns1/net1,net2",
			[]*types.NetworkSelectionElement{
				{
					Namespace:        "ns1",
					Name:             "net1",
					InterfaceRequest: "",
				},
				{
					Namespace:        "default",
					Name:             "net2",
					InterfaceRequest: "",
				},
			},
			false,
		),
		Entry(
			"csv - incorrect format forward slashes",
			"ns1/net1/if1",
			emptyList,
			true,
		),
		Entry(
			"csv - incorrect format @'s",
			"net1@if1@if2",
			emptyList,
			true,
		),
		Entry(
			"csv - incorrect mixed with correct",
			"ns/net1,net2,net3@if1@if2",
			emptyList,
			true,
		),
		Entry(
			"json - not an array",
			`{"name": "net1"}`,
			emptyList,
			true,
		),
		Entry(
			"json - correct example",
			`[{"name": "net1"},{"name": "net2", "namespace": "ns1"}]`,
			[]*types.NetworkSelectionElement{
				{
					Namespace:        "default",
					Name:             "net1",
					InterfaceRequest: "",
				},
				{
					Namespace:        "ns1",
					Name:             "net2",
					InterfaceRequest: "",
				},
			},
			false,
		),
	)
})
