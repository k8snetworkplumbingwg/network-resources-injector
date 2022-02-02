package userdefinedinjections

import (
	"encoding/json"
	"reflect"
	"strings"
	"sync"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"

	"github.com/k8snetworkplumbingwg/network-resources-injector/pkg/types"
)

const (
	userDefinedInjectionsMainKey = "user-defined-injections"
)

// UserDefinedInjections user defined injections
type UserDefinedInjections struct {
	sync.Mutex
	Patchs map[string]types.JsonPatchOperation
}

// CreateUserInjectionsStructure returns empty UserDefinedInjections structure
func CreateUserInjectionsStructure() *UserDefinedInjections {
	var userDefinedInjects = UserDefinedInjections{Patchs: make(map[string]types.JsonPatchOperation)}
	return &userDefinedInjects
}

// SetUserDefinedInjections sets additional injections to be applied in Pod spec
func (userDefinedInjects *UserDefinedInjections) SetUserDefinedInjections(injectionsCm *corev1.ConfigMap) {
	if v, fileExists := injectionsCm.Data[types.ConfigMapMainFileKey]; fileExists {
		var obj map[string]json.RawMessage
		var err error
		if err = json.Unmarshal([]byte(v), &obj); err != nil {
			glog.Warningf("Error during json unmarshal of main: %v", err)
			return
		}

		if userDefinedInjections, mainExists := obj[userDefinedInjectionsMainKey]; mainExists {
			var userDefinedInjectionsObj map[string]json.RawMessage
			if err = json.Unmarshal([]byte(userDefinedInjections), &userDefinedInjectionsObj); err != nil {
				glog.Warningf("Error during json unmarshal of injections: %v", err)
				return
			}

			// lock for writing
			userDefinedInjects.Lock()
			defer userDefinedInjects.Unlock()

			var patch types.JsonPatchOperation
			var userDefinedPatchs = userDefinedInjects.Patchs

			for k, value := range userDefinedInjectionsObj {
				existValue, exists := userDefinedPatchs[k]
				// unmarshal userDefined injection to json patch
				err := json.Unmarshal([]byte(value), &patch)
				if err != nil {
					glog.Errorf("Failed to unmarshal user-defined injection: %v", v)
					continue
				}
				// metadata.Annotations is the only supported field for user definition
				// jsonPatchOperation.Path should be "/metadata/annotations"
				if patch.Path != "/metadata/annotations" {
					glog.Errorf("Path: %v is not supported, only /metadata/annotations can be defined by user", patch.Path)
					continue
				}

				if !exists || !reflect.DeepEqual(existValue, patch) {
					glog.Infof("Initializing user-defined injections with key: %v, value: %v", k, v)
					userDefinedPatchs[k] = patch
				}
			}

			// remove stale entries from userDefined configMap
			for k := range userDefinedPatchs {
				if _, ok := userDefinedInjectionsObj[k]; ok {
					continue
				}
				glog.Infof("Removing stale entry: %v from user-defined injections", k)
				delete(userDefinedPatchs, k)
			}
		} else {
			glog.Warningf("Map does not contains [%s]. Clear old entries.", userDefinedInjectionsMainKey)
			userDefinedInjects.Patchs = make(map[string]types.JsonPatchOperation)
		}
	} else {
		glog.Warningf("Map does not contains [%s]. Clear old entries", types.ConfigMapMainFileKey)
		userDefinedInjects.Patchs = make(map[string]types.JsonPatchOperation)
	}
}

// CreateUserDefinedPatch creates customized patch for the specified POD
func (userDefinedInjects *UserDefinedInjections) CreateUserDefinedPatch(pod corev1.Pod) ([]types.JsonPatchOperation, error) {
	var userDefinedPatch []types.JsonPatchOperation

	// lock for reading
	userDefinedInjects.Lock()
	defer userDefinedInjects.Unlock()

	for k, v := range userDefinedInjects.Patchs {
		// The userDefinedInjects will be injected when:
		// 1. Pod labels contain the patch key defined in userDefinedInjects
		// 2. The value of patch key in pod labels(not in userDefinedInjects) is "true"
		if podValue, exists := pod.ObjectMeta.Labels[k]; exists && strings.ToLower(podValue) == "true" {
			userDefinedPatch = append(userDefinedPatch, v)
		}
	}

	return userDefinedPatch, nil
}
