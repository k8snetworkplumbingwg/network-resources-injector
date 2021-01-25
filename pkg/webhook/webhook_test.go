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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"

	"k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"gopkg.in/intel/multus-cni.v3/types"
)

var _ = Describe("Webhook", func() {

	Describe("Preparing Admission Review Response", func() {
		Context("Admission Review Request is nil", func() {
			It("should return error", func() {
				ar := &v1beta1.AdmissionReview{}
				ar.Request = nil
				Expect(prepareAdmissionReviewResponse(false, "", ar)).To(HaveOccurred())
			})
		})
		Context("Message is not empty", func() {
			It("should set message in the response", func() {
				ar := &v1beta1.AdmissionReview{}
				ar.Request = &v1beta1.AdmissionRequest{
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
				ar := &v1beta1.AdmissionReview{}
				ar.Request = &v1beta1.AdmissionRequest{}
				_, err := deserializeNetworkAttachmentDefinition(ar)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Deserializing Pod", func() {
		Context("It's not a Pod", func() {
			It("should return an error", func() {
				ar := &v1beta1.AdmissionReview{}
				ar.Request = &v1beta1.AdmissionRequest{}
				_, err := deserializePod(ar)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Writing a response", func() {
		Context("with an AdmissionReview", func() {
			It("should be marshalled and written to a HTTP Response Writer", func() {
				w := httptest.NewRecorder()
				ar := &v1beta1.AdmissionReview{}
				ar.Response = &v1beta1.AdmissionResponse{
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
		Context("Request body is empty", func() {
			It("mutate - should return an error", func() {
				req := httptest.NewRequest("POST", fmt.Sprintf("https://fakewebhook/mutate"), nil)
				w := httptest.NewRecorder()
				MutateHandler(w, req)
				resp := w.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		Context("Content type is not application/json", func() {
			It("mutate - should return an error", func() {
				req := httptest.NewRequest("POST", fmt.Sprintf("https://fakewebhook/mutate"), bytes.NewBufferString("fake-body"))
				req.Header.Set("Content-Type", "invalid-type")
				w := httptest.NewRecorder()
				MutateHandler(w, req)
				resp := w.Result()
				Expect(resp.StatusCode).To(Equal(http.StatusUnsupportedMediaType))
			})
		})
	})

	DescribeTable("Create customized patchs",

		func(pod corev1.Pod, cusInjectPatchs map[string]jsonPatchOperation, out []jsonPatchOperation) {
			cusInjects.Patchs = cusInjectPatchs
			appliedPatchs, _ := createCustomizedPatch(pod)
			Expect(appliedPatchs).Should(Equal(out))
		},
		Entry(
			"match pod label",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Labels: map[string]string{"nri-inject-annotation": "true"},
				},
				Spec: corev1.PodSpec{},
			},
			map[string]jsonPatchOperation{
				"nri-inject-annotation": jsonPatchOperation{
					Operation: "add",
					Path: "/metadata/annotations",
					Value: map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net"},
				},
			},
			[]jsonPatchOperation{
				{
					Operation: "add",
					Path: "/metadata/annotations",
					Value: map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net"},
				},
			},
		),
		Entry(
			"doesn't match pod label value",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Labels: map[string]string{"nri-inject-annotation": "false"},
				},
				Spec: corev1.PodSpec{},
			},
			map[string]jsonPatchOperation{
				"nri-inject-annotation": jsonPatchOperation{
					Operation: "add",
					Path: "/metadata/annotations",
					Value: map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net"},
				},
			},
			nil,
		),
		Entry(
			"doesn't match pod label key",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Labels: map[string]string{"nri-inject-labels": "true"},
				},
				Spec: corev1.PodSpec{},
			},
			map[string]jsonPatchOperation{
				"nri-inject-annotation": jsonPatchOperation{
					Operation: "add",
					Path: "/metadata/annotations",
					Value: map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net"},
				},
			},
			nil,
		),
	)

	DescribeTable("Get network selections",

		func(annotateKey string, pod corev1.Pod, patchs []jsonPatchOperation, out string, shouldExist bool) {
			nets, exist := getNetworkSelections(annotateKey, pod, patchs)
			Expect(exist).To(Equal(shouldExist))
			Expect(nets).Should(Equal(out))
		},
		Entry(
			"get from pod original annotation",
			"k8s.v1.cni.cncf.io/networks",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{"k8s.v1.cni.cncf.io/networks": "sriov-net"},
				},
				Spec: corev1.PodSpec{},
			},
			[]jsonPatchOperation{
				{
					Operation: "add",
					Path: "/metadata/annotations",
					Value: map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net-custom"},
				},
			},
			"sriov-net",
			true,
		),
		Entry(
			"get from pod customized annotation",
			"k8s.v1.cni.cncf.io/networks",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{"v1.multus-cni.io/default-network": "sriov-net"},
				},
				Spec: corev1.PodSpec{},
			},
			[]jsonPatchOperation{
				{
					Operation: "add",
					Path: "/metadata/annotations",
					Value: map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net-custom"},
				},
			},
			"sriov-net-custom",
			true,
		),
		Entry(
			"get from pod customized annotation",
			"k8s.v1.cni.cncf.io/networks",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
					Annotations: map[string]string{"v1.multus-cni.io/default-network": "sriov-net"},
				},
				Spec: corev1.PodSpec{},
			},
			[]jsonPatchOperation{
				{
					Operation: "add",
					Path: "/metadata/annotations",
					Value: map[string]interface{}{"v1.multus-cni.io/default-network": "sriov-net-custom"},
				},
			},
			"",
			false,
		),
	)

	DescribeTable("Setting customized injections",

		func(in *corev1.ConfigMap, existing map[string]jsonPatchOperation, out map[string]jsonPatchOperation) {
			SetCustomizedInjections(in)
			Expect(cusInjects.Patchs).Should(Equal(out))
		},
		Entry(
			"patch - empty config map",
			&corev1.ConfigMap{
				Data: map[string]string{},
			},
			map[string]jsonPatchOperation{},
			map[string]jsonPatchOperation{},
		),
		Entry(
			"patch - addtional networks annotation",
			&corev1.ConfigMap{
				Data: map[string]string{
					"nri-inject-annotation": "{\"op\": \"add\", \"path\": \"/metadata/annotations\", \"value\": {\"k8s.v1.cni.cncf.io/networks\": \"sriov-net\"}}"},
			},
			map[string]jsonPatchOperation{},
			map[string]jsonPatchOperation{
				"nri-inject-annotation": jsonPatchOperation{
					Operation: "add",
					Path: "/metadata/annotations",
					Value: map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net"},
				},
			},
		),
		Entry(
			"patch - default network annotation",
			&corev1.ConfigMap{
				Data: map[string]string{
					"nri-inject-annotation": "{\"op\": \"add\", \"path\": \"/metadata/annotations\", \"value\": {\"v1.multus-cni.io/default-network\": \"sriov-net\"}}"},
			},
			map[string]jsonPatchOperation{},
			map[string]jsonPatchOperation{
				"nri-inject-annotation": jsonPatchOperation{
					Operation: "add",
					Path: "/metadata/annotations",
					Value: map[string]interface{}{"v1.multus-cni.io/default-network": "sriov-net"},
				},
			},
		),
		Entry(
			"patch - non-annotation",
			&corev1.ConfigMap{
				Data: map[string]string{
					"nri-inject-labels": "{\"op\": \"add\", \"path\": \"/metadata/labels\", \"value\": {\"v1.multus-cni.io/default-network\": \"sriov-net\"}}",
				},
			},
			map[string]jsonPatchOperation{},
			map[string]jsonPatchOperation{},
		),
		Entry(
			"patch - remove stale entry",
			&corev1.ConfigMap{
				Data: map[string]string{},
			},
			map[string]jsonPatchOperation{
				"nri-inject-annotation": jsonPatchOperation{
					Operation: "add",
					Path: "/metadata/annotations",
					Value: map[string]interface{}{"v1.multus-cni.io/default-network": "sriov-net"},
				},
			},
			map[string]jsonPatchOperation{},
		),
		Entry(
			"patch - overwrite existing cusInjects",
			&corev1.ConfigMap{
				Data: map[string]string{
					"nri-inject-annotation": "{\"op\": \"add\", \"path\": \"/metadata/annotations\", \"value\": {\"v1.multus-cni.io/default-network\": \"sriov-net-new\"}}"},
			},
			map[string]jsonPatchOperation{
				"nri-inject-annotation": jsonPatchOperation{
					Operation: "add",
					Path: "/metadata/annotations",
					Value: map[string]interface{}{"v1.multus-cni.io/default-network": "sriov-net-old"},
				},
			},
			map[string]jsonPatchOperation{
				"nri-inject-annotation": jsonPatchOperation{
					Operation: "add",
					Path: "/metadata/annotations",
					Value: map[string]interface{}{"v1.multus-cni.io/default-network": "sriov-net-new"},
				},
			},
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
