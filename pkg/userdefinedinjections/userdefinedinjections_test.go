// Copyright (c) 2021 Intel, Redhat Corporation
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

package userdefinedinjections

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k8snetworkplumbingwg/network-resources-injector/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("UserDefinedInjections", func() {
	DescribeTable("Create user-defined patchs",

		func(pod corev1.Pod, userDefinedInjectPatchs map[string]types.JSONPatchOperation, out []types.JSONPatchOperation) {
			userDefinedInjects := CreateUserInjectionsStructure()
			userDefinedInjects.Patchs = userDefinedInjectPatchs
			appliedPatchs, _ := userDefinedInjects.CreateUserDefinedPatch(pod)
			Expect(appliedPatchs).Should(Equal(out))
		},
		Entry(
			"match pod label",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{"nri-inject-annotation": "true"},
				},
				Spec: corev1.PodSpec{},
			},
			map[string]types.JSONPatchOperation{
				"nri-inject-annotation": {
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net"},
				},
			},
			[]types.JSONPatchOperation{
				{
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net"},
				},
			},
		),
		Entry(
			"doesn't match pod label value",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{"nri-inject-annotation": "false"},
				},
				Spec: corev1.PodSpec{},
			},
			map[string]types.JSONPatchOperation{
				"nri-inject-annotation": {
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net"},
				},
			},
			nil,
		),
		Entry(
			"doesn't match pod label key",
			corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:   "test",
					Labels: map[string]string{"nri-inject-labels": "true"},
				},
				Spec: corev1.PodSpec{},
			},
			map[string]types.JSONPatchOperation{
				"nri-inject-annotation": {
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net"},
				},
			},
			nil,
		),
	)

	DescribeTable("Setting user-defined injections",
		func(in *corev1.ConfigMap, existing map[string]types.JSONPatchOperation, out map[string]types.JSONPatchOperation) {
			userDefinedInjects := CreateUserInjectionsStructure()
			userDefinedInjects.Patchs = existing
			userDefinedInjects.SetUserDefinedInjections(in)
			Expect(userDefinedInjects.Patchs).Should(Equal(out))
		},
		Entry(
			"patch - empty config map",
			&corev1.ConfigMap{
				Data: map[string]string{},
			},
			map[string]types.JSONPatchOperation{},
			map[string]types.JSONPatchOperation{},
		),
		Entry(
			"patch - empty config map",
			&corev1.ConfigMap{
				Data: map[string]string{},
			},
			map[string]types.JSONPatchOperation{
				"nri-inject-annotation": {
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"v1.multus-cni.io/default-network": "sriov-net"},
				},
			},
			map[string]types.JSONPatchOperation{},
		),
		Entry(
			"patch - config map without main config.json key",
			&corev1.ConfigMap{
				Data: map[string]string{
					"config": "{\"user-defined-injections\": { \"nri-inject-annotation\": {\"op\": \"add\", \"path\": \"/metadata/annotations\", \"value\": { \"k8s.v1.cni.cncf.io/networks\": \"sriov-net\" }}}}"},
			},
			map[string]types.JSONPatchOperation{},
			map[string]types.JSONPatchOperation{},
		),
		Entry(
			"patch - config map without main config.json key",
			&corev1.ConfigMap{
				Data: map[string]string{
					"config": "{\"user-defined-injections\": { \"nri-inject-annotation\": {\"op\": \"add\", \"path\": \"/metadata/annotations\", \"value\": { \"k8s.v1.cni.cncf.io/networks\": \"sriov-net\" }}}}"},
			},
			map[string]types.JSONPatchOperation{
				"nri-inject-annotation": {
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"v1.multus-cni.io/default-network": "sriov-net"},
				},
			},
			map[string]types.JSONPatchOperation{},
		),
		Entry(
			"patch - config map without userdefinedinjections key",
			&corev1.ConfigMap{
				Data: map[string]string{
					"config.json": "{\"custom\": { \"nri-inject-annotation\": {\"op\": \"add\", \"path\": \"/metadata/annotations\", \"value\": { \"k8s.v1.cni.cncf.io/networks\": \"sriov-net\" }}}}"},
			},
			map[string]types.JSONPatchOperation{},
			map[string]types.JSONPatchOperation{},
		),
		Entry(
			"patch - config map with errors in json path",
			&corev1.ConfigMap{
				Data: map[string]string{
					"config.json": "{\"user-defined-injections\": { \"nri-inject-annotation\": {\"op\": 5, \"path\": \"/metadata/annotations\", \"value\": { \"k8s.v1.cni.cncf.io/networks\": \"sriov-net\" }}}}"},
			},
			map[string]types.JSONPatchOperation{},
			map[string]types.JSONPatchOperation{},
		),
		Entry(
			"patch - config map with incorrect json path - not /metadata/annotations",
			&corev1.ConfigMap{
				Data: map[string]string{
					"config.json": "{\"user-defined-injections\": { \"nri-inject-annotation\": {\"op\": \"add\", \"path\": \"/metadata/supprise\", \"value\": { \"k8s.v1.cni.cncf.io/networks\": \"sriov-net\" }}}}"},
			},
			map[string]types.JSONPatchOperation{},
			map[string]types.JSONPatchOperation{},
		),
		Entry(
			"patch - additional networks annotation",
			&corev1.ConfigMap{
				Data: map[string]string{
					"config.json": "{\"user-defined-injections\": { \"nri-inject-annotation\": {\"op\": \"add\", \"path\": \"/metadata/annotations\", \"value\": { \"k8s.v1.cni.cncf.io/networks\": \"sriov-net\" }}}}"},
			},
			map[string]types.JSONPatchOperation{},
			map[string]types.JSONPatchOperation{
				"nri-inject-annotation": {
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"k8s.v1.cni.cncf.io/networks": "sriov-net"},
				},
			},
		),
		Entry(
			"patch - default network annotation",
			&corev1.ConfigMap{
				Data: map[string]string{
					"config.json": "{\"user-defined-injections\": { \"nri-inject-annotation\": {\"op\": \"add\", \"path\": \"/metadata/annotations\", \"value\": {\"v1.multus-cni.io/default-network\": \"sriov-net\" }}}}"},
			},
			map[string]types.JSONPatchOperation{},
			map[string]types.JSONPatchOperation{
				"nri-inject-annotation": {
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"v1.multus-cni.io/default-network": "sriov-net"},
				},
			},
		),
		Entry(
			"patch - remove stale entry",
			&corev1.ConfigMap{
				Data: map[string]string{
					"config.json": "{\"user-defined-injections\": { }}"},
			},
			map[string]types.JSONPatchOperation{
				"nri-inject-annotation": {
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"v1.multus-cni.io/default-network": "sriov-net"},
				},
			},
			map[string]types.JSONPatchOperation{},
		),
		Entry(
			"patch - overwrite existing userDefinedInjects",
			&corev1.ConfigMap{
				Data: map[string]string{
					"config.json": "{\"user-defined-injections\": { \"nri-inject-annotation\": {\"op\": \"add\", \"path\": \"/metadata/annotations\", \"value\": {\"v1.multus-cni.io/default-network\": \"sriov-net-new\"}}}}"},
			},
			map[string]types.JSONPatchOperation{
				"nri-inject-annotation": {
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"v1.multus-cni.io/default-network": "sriov-net-old"},
				},
			},
			map[string]types.JSONPatchOperation{
				"nri-inject-annotation": {
					Operation: "add",
					Path:      "/metadata/annotations",
					Value:     map[string]interface{}{"v1.multus-cni.io/default-network": "sriov-net-new"},
				},
			},
		),
	)
})
