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

// +build unittests

// This file should be only include in build system during unit tests execution.
// Special method allows to setup structure with values needed by test.

package controlswitches

func SetupControlSwitchesUnitTests(downAPI, honor *bool, name *string) *ControlSwitches {
	var initFlags ControlSwitches

	initFlags.injectHugepageDownAPI = downAPI
	initFlags.resourceNameKeysFlag = name
	initFlags.resourcesHonorFlag = honor

	return &initFlags
}
