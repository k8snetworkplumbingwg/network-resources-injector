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
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"

	"github.com/k8snetworkplumbingwg/network-resources-injector/pkg/types"
)

const (
	// control switch keys
	controlSwitchesMainKey = "features"

	// enableHugePageDownAPIKey feature name
	enableHugePageDownAPIKey = "enableHugePageDownApi"
	// enableHonorExistingResourcesKey feature name
	enableHonorExistingResourcesKey = "enableHonorExistingResources"
)

// controlSwitchesStates - depicts possible feature states
type controlSwitchesStates struct {
	active  bool
	initial bool
}

// setActiveToInitialState - set active state to the initial state set during initialization
func (state *controlSwitchesStates) setActiveToInitialState() {
	state.active = state.initial
}

// setActiveState - set active state to the passed value
func (state *controlSwitchesStates) setActiveState(value bool) {
	state.active = value
}

type ControlSwitches struct {
	// pointers to command line arguments
	injectHugepageDownAPI *bool
	resourceNameKeysFlag  *string
	resourcesHonorFlag    *bool

	configuration    map[string]controlSwitchesStates
	resourceNameKeys []string
	isValid          bool
}

// SetupControlSwitchesFlags - setup all control switches flags that can be set as command line NRI arguments
// :return pointer to the structure that should be initialized with InitControlSwitches
func SetupControlSwitchesFlags() *ControlSwitches {
	var initFlags ControlSwitches

	initFlags.injectHugepageDownAPI = flag.Bool("injectHugepageDownApi", false, "Enable hugepage requests and limits into Downward API.")
	initFlags.resourceNameKeysFlag = flag.String("network-resource-name-keys", "k8s.v1.cni.cncf.io/resourceName", "comma separated resource name keys --network-resource-name-keys.")
	initFlags.resourcesHonorFlag = flag.Bool("honor-resources", false, "Honor the existing requested resources requests & limits --honor-resources")

	return &initFlags
}

// InitControlSwitches - initialize internal control switches structures based on command line arguments
func (switches *ControlSwitches) InitControlSwitches() {
	switches.configuration = make(map[string]controlSwitchesStates)

	state := controlSwitchesStates{initial: *switches.injectHugepageDownAPI, active: *switches.injectHugepageDownAPI}
	switches.configuration[enableHugePageDownAPIKey] = state

	state = controlSwitchesStates{initial: *switches.resourcesHonorFlag, active: *switches.resourcesHonorFlag}
	switches.configuration[enableHonorExistingResourcesKey] = state

	switches.resourceNameKeys = setResourceNameKeys(*switches.resourceNameKeysFlag)

	switches.isValid = true
}

// setResourceNameKeys extracts resources from a string and add them to resourceNameKeys array
func setResourceNameKeys(keys string) []string {
	var resourceNameKeys []string

	for _, resourceNameKey := range strings.Split(keys, ",") {
		resourceNameKey = strings.TrimSpace(resourceNameKey)
		resourceNameKeys = append(resourceNameKeys, resourceNameKey)
	}

	return resourceNameKeys
}

func (switches *ControlSwitches) GetResourceNameKeys() []string {
	return switches.resourceNameKeys
}

func (switches *ControlSwitches) IsHugePagedownAPIEnabled() bool {
	return switches.configuration[enableHugePageDownAPIKey].active
}

func (switches *ControlSwitches) IsHonorExistingResourcesEnabled() bool {
	return switches.configuration[enableHonorExistingResourcesKey].active
}

func (switches *ControlSwitches) IsResourcesNameEnabled() bool {
	return len(*switches.resourceNameKeysFlag) > 0
}

// IsValid returns true when ControlSwitches structure was initialized, false otherwise
func (switches *ControlSwitches) IsValid() bool {
	return switches.isValid
}

// GetAllFeaturesState returns string with information if feature is active or not
func (switches *ControlSwitches) GetAllFeaturesState() string {
	var output string

	output = fmt.Sprintf("HugePageInject: %t", switches.IsHugePagedownAPIEnabled())
	output = output + " / " + fmt.Sprintf("HonorExistingResources: %t", switches.IsHonorExistingResourcesEnabled())
	output = output + " / " + fmt.Sprintf("EnableResourceNames: %t", switches.IsResourcesNameEnabled())

	return output
}

// setAllFeaturesToInitialState - reset feature state to initial one set during NRI initialization
func (switches *ControlSwitches) setAllFeaturesToInitialState() {
	state := switches.configuration[enableHugePageDownAPIKey]
	state.setActiveToInitialState()
	switches.configuration[enableHugePageDownAPIKey] = state

	state = switches.configuration[enableHonorExistingResourcesKey]
	state.setActiveToInitialState()
	switches.configuration[enableHonorExistingResourcesKey] = state
}

// setFeatureToState set given feature to the state defined in the map object
func (switches *ControlSwitches) setFeatureToState(featureName string, switchObj map[string]bool) {
	if featureState, available := switchObj[featureName]; available {
		state := switches.configuration[featureName]
		state.setActiveState(featureState)
		switches.configuration[featureName] = state
	} else {
		state := switches.configuration[featureName]
		state.setActiveToInitialState()
		switches.configuration[featureName] = state
	}
}

// ProcessControlSwitchesConfigMap sets on the fly control switches
// :param controlSwitchesCm - Kubernetes ConfigMap with control switches definition
func (switches *ControlSwitches) ProcessControlSwitchesConfigMap(controlSwitchesCm *corev1.ConfigMap) {
	var err error
	if v, fileExists := controlSwitchesCm.Data[types.ConfigMapMainFileKey]; fileExists {
		var obj map[string]json.RawMessage

		if err = json.Unmarshal([]byte(v), &obj); err != nil {
			glog.Warningf("Error during json unmarshal %v", err)
			switches.setAllFeaturesToInitialState()
			return
		}

		if controlSwitches, mainExists := obj[controlSwitchesMainKey]; mainExists {
			var switchObj map[string]bool

			if err = json.Unmarshal(controlSwitches, &switchObj); err != nil {
				glog.Warningf("Unable to unmarshal [%s] from configmap, err: %v", controlSwitchesMainKey, err)
				switches.setAllFeaturesToInitialState()
				return
			}

			switches.setFeatureToState(enableHugePageDownAPIKey, switchObj)
			switches.setFeatureToState(enableHonorExistingResourcesKey, switchObj)
		} else {
			glog.Warningf("Map does not contains [%s]", controlSwitchesMainKey)
		}
	} else {
		glog.Warningf("Map does not contains [%s]", types.ConfigMapMainFileKey)
	}
}
