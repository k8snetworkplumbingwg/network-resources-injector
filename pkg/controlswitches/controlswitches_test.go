// Copyright (c) 2021 Intel Corporation
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

package controlswitches

import (
	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// Helper functions

func createBool(value bool) *bool {
	return &value
}

func createString(value string) *string {
	return &value
}

var _ = Describe("Verify controlswitches package", func() {
	var structure *ControlSwitches

	Describe("Common functions", func() {
		Context("Display features", func() {
			BeforeEach(func() {
				structure = SetupControlSwitchesUnitTests(createBool(false), createBool(false), createString(""))
				structure.InitControlSwitches()
			})

			AfterEach(func() {
				structure = nil
			})

			It("Setup structure", func() {
				structure = SetupControlSwitchesFlags()
				Expect(structure.IsValid()).Should(Equal(false))

				structure.InitControlSwitches()
				Expect(structure.IsValid()).Should(Equal(true))
			})
		})
	})

	Describe("Verify state structure", func() {
		Context("Check", func() {
			It("Set explicit active state", func() {
				state := controlSwitchesStates{active: false, initial: false}

				state.setActiveState(true)
				Expect(state.active).Should(Equal(true))
				Expect(state.initial).Should(Equal(false))

				state.setActiveState(false)
				Expect(state.active).Should(Equal(false))
				Expect(state.initial).Should(Equal(false))
			})

			It("Set active to initial when initial is true", func() {
				state := controlSwitchesStates{active: false, initial: true}

				state.setActiveToInitialState()
				Expect(state.active).Should(Equal(true))
				Expect(state.initial).Should(Equal(true))
			})

			It("Set active to initial when initial is false", func() {
				state := controlSwitchesStates{active: true, initial: false}

				state.setActiveToInitialState()
				Expect(state.active).Should(Equal(false))
				Expect(state.initial).Should(Equal(false))
			})
		})
	})

	Describe("Hugepages downward API", func() {
		Context("Feature configuration flags", func() {
			AfterEach(func() {
				structure = nil
			})

			It("Feature set to false, other features set to true", func() {
				structure = SetupControlSwitchesUnitTests(createBool(false), createBool(true), createString("something"))
				structure.InitControlSwitches()

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(false))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(true))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(true))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(true))
			})

			It("Feature set to true, other features set to false", func() {
				structure = SetupControlSwitchesUnitTests(createBool(true), createBool(false), createString(""))
				structure.InitControlSwitches()

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(true))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(false))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(true))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))
			})
		})
	})

	Describe("Honor existing resources", func() {
		Context("Feature configuration flags", func() {
			AfterEach(func() {
				structure = nil
			})

			It("Feature set to false, other features set to true", func() {
				structure = SetupControlSwitchesUnitTests(createBool(true), createBool(false), createString("something"))
				structure.InitControlSwitches()

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(true))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(false))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(true))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(true))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))
			})

			It("Feature set to true, other features set to false", func() {
				structure = SetupControlSwitchesUnitTests(createBool(false), createBool(true), createString(""))
				structure.InitControlSwitches()

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(false))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(true))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(true))
			})
		})
	})

	Describe("Inject resource name keys ", func() {
		Context("Feature configuration flags", func() {
			AfterEach(func() {
				structure = nil
			})

			It("Feature set to false, other features set to true", func() {
				structure = SetupControlSwitchesUnitTests(createBool(true), createBool(true), createString(""))
				structure.InitControlSwitches()

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(true))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(true))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(true))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(true))

				Expect(structure.GetResourceNameKeys()).Should(Equal([]string{""}))
			})

			It("Feature set to true, other features set to false", func() {
				structure = SetupControlSwitchesUnitTests(createBool(false), createBool(false), createString("something"))
				structure.InitControlSwitches()

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(false))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(false))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(true))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))

				Expect(structure.GetResourceNameKeys()).Should(Equal([]string{"something"}))
			})
		})
	})

	Describe("User defined injections", func() {
		Context("Feature configuration flags", func() {
			AfterEach(func() {
				structure = nil
			})

			It("Feature set to false, other features set to true", func() {
				structure = SetupControlSwitchesUnitTests(createBool(true), createBool(true), createString("something"))
				structure.InitControlSwitches()

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(true))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(true))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(true))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(true))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(true))
			})

			It("Feature set to true, other features set to false", func() {
				structure = SetupControlSwitchesUnitTests(createBool(false), createBool(false), createString(""))
				structure.InitControlSwitches()

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(false))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(false))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))
			})
		})
	})

	Describe("Process Control Switches config map", func() {
		Context("Map without [features]", func() {
			BeforeEach(func() {
				structure = SetupControlSwitchesUnitTests(createBool(false), createBool(false), createString(""))
				structure.InitControlSwitches()
			})

			AfterEach(func() {
				structure = nil
			})

			It("Missing key", func() {
				cm := corev1.ConfigMap{
					Data: map[string]string{},
				}

				structure.ProcessControlSwitchesConfigMap(&cm)

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(false))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(false))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))
			})

			It("Map without config.json key", func() {
				cm := corev1.ConfigMap{
					Data: map[string]string{"nri-inject-annotation": "true"},
				}

				structure.ProcessControlSwitchesConfigMap(&cm)

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(false))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(false))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))
			})

			It("Map with correct key, but without [features] inside", func() {
				const value = `{
							"networkResourceNameKeys": ["k8s.v1.cni.cncf.io/resourceName", "k8s.v1.cni.cncf.io/bridgeName"],
							"customInjection": {
								"network-resource-injector-pod-annotation": {
									"op": "add",
								 	"path": "/metadata/annotations",
								  	"value": {
										  "k8s.v1.cni.cncf.io/networks": "sriov-net-attach-def"
									}
								}
							}
						}
						`

				cm := corev1.ConfigMap{
					Data: map[string]string{"config.json": value},
				}

				structure.ProcessControlSwitchesConfigMap(&cm)

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(false))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(false))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))
			})

			It("Map with correct key, with [features] inside, but features name are incorrect", func() {
				const value = `{
							"features": {
								"enableHugePageDown": false,
								"enableHonorExisting": true,
								"enableCustomizedInje": false,
								"enableResourceNa": false
							},
							"networkResourceNameKeys": ["k8s.v1.cni.cncf.io/resourceName", "k8s.v1.cni.cncf.io/bridgeName"],
							"customInjection": {
								"network-resource-injector-pod-annotation": {
									"op": "add",
								 	"path": "/metadata/annotations",
								  	"value": {
										  "k8s.v1.cni.cncf.io/networks": "sriov-net-attach-def"
									}
								}
							}
						}
						`
				cm := corev1.ConfigMap{
					Data: map[string]string{"config.json": value},
				}

				structure.ProcessControlSwitchesConfigMap(&cm)

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(false))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(false))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))
			})

			It("Map with correct key, with [features] inside - but JSON is invalid", func() {
				const value = `{
							"features": {
								"enableHugePageDownApi": false,
								"enableHonorExistingResources": true
							},
							"networkResourceNameKeys": ["k8s.v1.cni.cncf.io/resourceName", "k8s.v1.cni.cncf.io/bridgeName"],
							"customInjection": {
								"network-resource-injector-pod-annotation": {
									"op": "add",
								 	"path": "/metadata/annotations"
								  	"value": {
										  "k8s.v1.cni.cncf.io/networks": "sriov-net-attach-def"
									}
								}
							}
						}
						`
				cm := corev1.ConfigMap{
					Data: map[string]string{"config.json": value},
				}

				structure.ProcessControlSwitchesConfigMap(&cm)

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(false))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(false))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))
			})

			It("Map with correct key, with [features] inside - all set to false", func() {
				const value = `{
							"features": {
								"enableHugePageDownApi": false,
								"enableHonorExistingResources": false
							},
							"networkResourceNameKeys": ["k8s.v1.cni.cncf.io/resourceName", "k8s.v1.cni.cncf.io/bridgeName"],
							"customInjection": {
								"network-resource-injector-pod-annotation": {
									"op": "add",
								 	"path": "/metadata/annotations",
								  	"value": {
										  "k8s.v1.cni.cncf.io/networks": "sriov-net-attach-def"
									}
								}
							}
						}
						`
				cm := corev1.ConfigMap{
					Data: map[string]string{"config.json": value},
				}

				structure.ProcessControlSwitchesConfigMap(&cm)

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(false))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(false))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))
			})

			// set one by one, instead of all in one
			It("Map with correct key, with [features] inside - value of feature set to string instead of bool", func() {
				const value = `{
							"features": {
								"enableHugePageDownApi": true,
								"enableHonorExistingResources": "isThisAnError"
							},
							"networkResourceNameKeys": ["k8s.v1.cni.cncf.io/resourceName", "k8s.v1.cni.cncf.io/bridgeName"],
							"customInjection": {
								"network-resource-injector-pod-annotation": {
									"op": "add",
								 	"path": "/metadata/annotations",
								  	"value": {
										  "k8s.v1.cni.cncf.io/networks": "sriov-net-attach-def"
									}
								}
							}
						}
						`
				cm := corev1.ConfigMap{
					Data: map[string]string{"config.json": value},
				}

				structure.ProcessControlSwitchesConfigMap(&cm)

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(false))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(false))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))
			})

			It("Map with correct key, with [features] inside - all set to true", func() {
				const value = `{
							"features": {
								"enableHugePageDownApi": true,
								"enableHonorExistingResources": true
							},
							"networkResourceNameKeys": ["k8s.v1.cni.cncf.io/resourceName", "k8s.v1.cni.cncf.io/bridgeName"],
							"customInjection": {
								"network-resource-injector-pod-annotation": {
									"op": "add",
								 	"path": "/metadata/annotations",
								  	"value": {
										  "k8s.v1.cni.cncf.io/networks": "sriov-net-attach-def"
									}
								}
							}
						}
						`
				cm := corev1.ConfigMap{
					Data: map[string]string{"config.json": value},
				}

				structure.ProcessControlSwitchesConfigMap(&cm)

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(true))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(true))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))
			})

			It("Map with correct key, with [features] inside - mix with values true / false", func() {
				const value = `{
							"features": {
								"enableHugePageDownApi": true,
								"enableHonorExistingResources": false
							},
							"networkResourceNameKeys": ["k8s.v1.cni.cncf.io/resourceName", "k8s.v1.cni.cncf.io/bridgeName"],
							"customInjection": {
								"network-resource-injector-pod-annotation": {
									"op": "add",
								 	"path": "/metadata/annotations",
								  	"value": {
										  "k8s.v1.cni.cncf.io/networks": "sriov-net-attach-def"
									}
								}
							}
						}
						`
				cm := corev1.ConfigMap{
					Data: map[string]string{"config.json": value},
				}

				structure.ProcessControlSwitchesConfigMap(&cm)

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(true))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(false))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))
			})

			It("Map with correct key, with [features] inside - some features are missing", func() {
				// create map that does not have all features defined, expected to be removed
				const value = `{
							"features": {
								"enableHugePageDownApi": true
							}
						}
						`

				cm := corev1.ConfigMap{
					Data: map[string]string{"config.json": value},
				}

				structure.ProcessControlSwitchesConfigMap(&cm)

				Expect(structure.IsHugePagedownAPIEnabled()).Should(Equal(true))
				Expect(structure.IsHonorExistingResourcesEnabled()).Should(Equal(false))
				Expect(structure.IsResourcesNameEnabled()).Should(Equal(false))

				Expect(structure.configuration[enableHugePageDownAPIKey].initial).Should(Equal(false))
				Expect(structure.configuration[enableHonorExistingResourcesKey].initial).Should(Equal(false))
			})
		})
	})
})
