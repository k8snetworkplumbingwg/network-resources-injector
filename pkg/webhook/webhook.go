// Copyright (c) 2018 Intel Corporation
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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/golang/glog"
	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/pkg/errors"
	multus "gopkg.in/intel/multus-cni.v3/types"

	"github.com/k8snetworkplumbingwg/network-resources-injector/pkg/types"
	"k8s.io/api/admission/v1beta1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type jsonPatchOperation struct {
	Operation string      `json:"op"`
	Path      string      `json:"path"`
	Value     interface{} `json:"value,omitempty"`
}

type customInjections struct {
	sync.Mutex
	Patchs map[string]jsonPatchOperation
}

type hugepageResourceData struct {
	ResourceName  string
	ContainerName string
	Path          string
}

const (
	networksAnnotationKey       = "k8s.v1.cni.cncf.io/networks"
	nodeSelectorKey             = "k8s.v1.cni.cncf.io/nodeSelector"
	defaultNetworkAnnotationKey = "v1.multus-cni.io/default-network"
)

var (
	clientset              kubernetes.Interface
	injectHugepageDownApi  bool
	resourceNameKeys       []string
	honorExistingResources bool
	cusInjects             = &customInjections{Patchs: make(map[string]jsonPatchOperation)}
)

func prepareAdmissionReviewResponse(allowed bool, message string, ar *v1beta1.AdmissionReview) error {
	if ar.Request != nil {
		ar.Response = &v1beta1.AdmissionResponse{
			UID:     ar.Request.UID,
			Allowed: allowed,
		}
		if message != "" {
			ar.Response.Result = &metav1.Status{
				Message: message,
			}
		}
		return nil
	}
	return errors.New("received empty AdmissionReview request")
}

func readAdmissionReview(req *http.Request, w http.ResponseWriter) (*v1beta1.AdmissionReview, int, error) {
	var body []byte

	if req.Body != nil {
		req.Body = http.MaxBytesReader(w, req.Body, 1<<20)
		if data, err := ioutil.ReadAll(req.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		err := errors.New("Error reading HTTP request: empty body")
		glog.Errorf("%s", err)
		return nil, http.StatusBadRequest, err
	}

	/* validate HTTP request headers */
	contentType := req.Header.Get("Content-Type")
	if contentType != "application/json" {
		err := errors.Errorf("Invalid Content-Type='%s', expected 'application/json'", contentType)
		glog.Errorf("%v", err)
		return nil, http.StatusUnsupportedMediaType, err
	}

	/* read AdmissionReview from the request body */
	ar, err := deserializeAdmissionReview(body)
	if err != nil {
		err := errors.Wrap(err, "error deserializing AdmissionReview")
		glog.Errorf("%v", err)
		return nil, http.StatusBadRequest, err
	}

	return ar, http.StatusOK, nil
}

func deserializeAdmissionReview(body []byte) (*v1beta1.AdmissionReview, error) {
	ar := &v1beta1.AdmissionReview{}
	runtimeScheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(runtimeScheme)
	deserializer := codecs.UniversalDeserializer()
	_, _, err := deserializer.Decode(body, nil, ar)

	/* Decode() won't return an error if the data wasn't actual AdmissionReview */
	if err == nil && ar.TypeMeta.Kind != "AdmissionReview" {
		err = errors.New("received object is not an AdmissionReview")
	}

	return ar, err
}

func deserializeNetworkAttachmentDefinition(ar *v1beta1.AdmissionReview) (cniv1.NetworkAttachmentDefinition, error) {
	/* unmarshal NetworkAttachmentDefinition from AdmissionReview request */
	netAttachDef := cniv1.NetworkAttachmentDefinition{}
	err := json.Unmarshal(ar.Request.Object.Raw, &netAttachDef)
	return netAttachDef, err
}

func deserializePod(ar *v1beta1.AdmissionReview) (corev1.Pod, error) {
	/* unmarshal Pod from AdmissionReview request */
	pod := corev1.Pod{}
	err := json.Unmarshal(ar.Request.Object.Raw, &pod)
	if pod.ObjectMeta.Namespace != "" {
		return pod, err
	}
	ownerRef := pod.ObjectMeta.OwnerReferences
	if ownerRef != nil && len(ownerRef) > 0 {
		namespace, err := getNamespaceFromOwnerReference(pod.ObjectMeta.OwnerReferences[0])
		if err != nil {
			return pod, err
		}
		pod.ObjectMeta.Namespace = namespace
	}
	return pod, err
}

func getNamespaceFromOwnerReference(ownerRef metav1.OwnerReference) (namespace string, err error) {
	namespace = ""
	switch ownerRef.Kind {
	case "ReplicaSet":
		var replicaSets *v1.ReplicaSetList
		replicaSets, err = clientset.AppsV1().ReplicaSets("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return
		}
		for _, replicaSet := range replicaSets.Items {
			if replicaSet.ObjectMeta.Name == ownerRef.Name && replicaSet.ObjectMeta.UID == ownerRef.UID {
				namespace = replicaSet.ObjectMeta.Namespace
				err = nil
				break
			}
		}
	case "DaemonSet":
		var daemonSets *v1.DaemonSetList
		daemonSets, err = clientset.AppsV1().DaemonSets("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return
		}
		for _, daemonSet := range daemonSets.Items {
			if daemonSet.ObjectMeta.Name == ownerRef.Name && daemonSet.ObjectMeta.UID == ownerRef.UID {
				namespace = daemonSet.ObjectMeta.Namespace
				err = nil
				break
			}
		}
	case "StatefulSet":
		var statefulSets *v1.StatefulSetList
		statefulSets, err = clientset.AppsV1().StatefulSets("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return
		}
		for _, statefulSet := range statefulSets.Items {
			if statefulSet.ObjectMeta.Name == ownerRef.Name && statefulSet.ObjectMeta.UID == ownerRef.UID {
				namespace = statefulSet.ObjectMeta.Namespace
				err = nil
				break
			}
		}
	case "ReplicationController":
		var replicationControllers *corev1.ReplicationControllerList
		replicationControllers, err = clientset.CoreV1().ReplicationControllers("").List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return
		}
		for _, replicationController := range replicationControllers.Items {
			if replicationController.ObjectMeta.Name == ownerRef.Name && replicationController.ObjectMeta.UID == ownerRef.UID {
				namespace = replicationController.ObjectMeta.Namespace
				err = nil
				break
			}
		}
	default:
		glog.Infof("owner reference kind is not supported: %v, using default namespace", ownerRef.Kind)
		namespace = "default"
		return
	}

	if namespace == "" {
		err = errors.New("pod namespace is not found")
	}

	return

}

func toSafeJsonPatchKey(in string) string {
	out := strings.Replace(in, "~", "~0", -1)
	out = strings.Replace(out, "/", "~1", -1)
	return out
}

func parsePodNetworkSelections(podNetworks, defaultNamespace string) ([]*multus.NetworkSelectionElement, error) {
	var networkSelections []*multus.NetworkSelectionElement

	if len(podNetworks) == 0 {
		err := errors.New("empty string passed as network selection elements list")
		glog.Error(err)
		return nil, err
	}

	/* try to parse as JSON array */
	err := json.Unmarshal([]byte(podNetworks), &networkSelections)

	/* if failed, try to parse as comma separated */
	if err != nil {
		glog.Infof("'%s' is not in JSON format: %s... trying to parse as comma separated network selections list", podNetworks, err)
		for _, networkSelection := range strings.Split(podNetworks, ",") {
			networkSelection = strings.TrimSpace(networkSelection)
			networkSelectionElement, err := parsePodNetworkSelectionElement(networkSelection, defaultNamespace)
			if err != nil {
				err := errors.Wrap(err, "error parsing network selection element")
				glog.Error(err)
				return nil, err
			}
			networkSelections = append(networkSelections, networkSelectionElement)
		}
	}

	/* fill missing namespaces with default value */
	for _, networkSelection := range networkSelections {
		if networkSelection.Namespace == "" {
			networkSelection.Namespace = defaultNamespace
		}
	}

	return networkSelections, nil
}

func parsePodNetworkSelectionElement(selection, defaultNamespace string) (*multus.NetworkSelectionElement, error) {
	var namespace, name, netInterface string
	var networkSelectionElement *multus.NetworkSelectionElement

	units := strings.Split(selection, "/")
	switch len(units) {
	case 1:
		namespace = defaultNamespace
		name = units[0]
	case 2:
		namespace = units[0]
		name = units[1]
	default:
		err := errors.Errorf("invalid network selection element - more than one '/' rune in: '%s'", selection)
		glog.Info(err)
		return networkSelectionElement, err
	}

	units = strings.Split(name, "@")
	switch len(units) {
	case 1:
		name = units[0]
		netInterface = ""
	case 2:
		name = units[0]
		netInterface = units[1]
	default:
		err := errors.Errorf("invalid network selection element - more than one '@' rune in: '%s'", selection)
		glog.Info(err)
		return networkSelectionElement, err
	}

	validNameRegex, _ := regexp.Compile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	for _, unit := range []string{namespace, name, netInterface} {
		ok := validNameRegex.MatchString(unit)
		if !ok && len(unit) > 0 {
			err := errors.Errorf("at least one of the network selection units is invalid: error found at '%s'", unit)
			glog.Info(err)
			return networkSelectionElement, err
		}
	}

	networkSelectionElement = &multus.NetworkSelectionElement{
		Namespace:        namespace,
		Name:             name,
		InterfaceRequest: netInterface,
	}

	return networkSelectionElement, nil
}

func getNetworkAttachmentDefinition(namespace, name string) (*cniv1.NetworkAttachmentDefinition, error) {
	path := fmt.Sprintf("/apis/k8s.cni.cncf.io/v1/namespaces/%s/network-attachment-definitions/%s", namespace, name)
	rawNetworkAttachmentDefinition, err := clientset.ExtensionsV1beta1().RESTClient().Get().AbsPath(path).DoRaw(context.TODO())
	if err != nil {
		err := errors.Wrapf(err, "could not get Network Attachment Definition %s/%s", namespace, name)
		glog.Error(err)
		return nil, err
	}

	networkAttachmentDefinition := cniv1.NetworkAttachmentDefinition{}
	json.Unmarshal(rawNetworkAttachmentDefinition, &networkAttachmentDefinition)

	return &networkAttachmentDefinition, nil
}

func parseNetworkAttachDefinition(net *multus.NetworkSelectionElement, reqs map[string]int64, nsMap map[string]string) (map[string]int64, map[string]string, error) {
	/* for each network in annotation ask API server for network-attachment-definition */
	networkAttachmentDefinition, err := getNetworkAttachmentDefinition(net.Namespace, net.Name)
	if err != nil {
		/* if doesn't exist: deny pod */
		reason := errors.Wrapf(err, "could not find network attachment definition '%s/%s'", net.Namespace, net.Name)
		glog.Error(reason)
		return reqs, nsMap, reason
	}
	glog.Infof("network attachment definition '%s/%s' found", net.Namespace, net.Name)

	/* network object exists, so check if it contains resourceName annotation */
	for _, networkResourceNameKey := range resourceNameKeys {
		if resourceName, exists := networkAttachmentDefinition.ObjectMeta.Annotations[networkResourceNameKey]; exists {
			/* add resource to map/increment if it was already there */
			reqs[resourceName]++
			glog.Infof("resource '%s' needs to be requested for network '%s/%s'", resourceName, net.Namespace, net.Name)
		} else {
			glog.Infof("network '%s/%s' doesn't use custom resources, skipping...", net.Namespace, net.Name)
		}
	}

	/* parse the net-attach-def annotations for node selector label and add it to the desiredNsMap */
	if ns, exists := networkAttachmentDefinition.ObjectMeta.Annotations[nodeSelectorKey]; exists {
		nsNameValue := strings.Split(ns, "=")
		nsNameValueLen := len(nsNameValue)
		if nsNameValueLen > 2 {
			reason := fmt.Errorf("node selector in net-attach-def %s has more than one label", net.Name)
			glog.Error(reason)
			return reqs, nsMap, reason
		} else if nsNameValueLen == 2 {
			nsMap[strings.TrimSpace(nsNameValue[0])] = strings.TrimSpace(nsNameValue[1])
		} else {
			nsMap[strings.TrimSpace(ns)] = ""
		}
	}

	return reqs, nsMap, nil
}

func handleValidationError(w http.ResponseWriter, ar *v1beta1.AdmissionReview, orgErr error) {
	err := prepareAdmissionReviewResponse(false, orgErr.Error(), ar)
	if err != nil {
		err := errors.Wrap(err, "error preparing AdmissionResponse")
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeResponse(w, ar)
}

func writeResponse(w http.ResponseWriter, ar *v1beta1.AdmissionReview) {
	glog.Infof("sending response to the Kubernetes API server")
	resp, _ := json.Marshal(ar)
	w.Write(resp)
}

func patchEmptyResources(patch []jsonPatchOperation, containerIndex uint, key string) []jsonPatchOperation {
	patch = append(patch, jsonPatchOperation{
		Operation: "add",
		Path:      "/spec/containers/" + fmt.Sprintf("%d", containerIndex) + "/resources/" + toSafeJsonPatchKey(key),
		Value:     corev1.ResourceList{},
	})
	return patch
}

func addVolDownwardAPI(patch []jsonPatchOperation, hugepageResourceList []hugepageResourceData) []jsonPatchOperation {
	labels := corev1.ObjectFieldSelector{
		FieldPath: "metadata.labels",
	}
	dAPILabels := corev1.DownwardAPIVolumeFile{
		Path:     types.LabelsPath,
		FieldRef: &labels,
	}
	annotations := corev1.ObjectFieldSelector{
		FieldPath: "metadata.annotations",
	}
	dAPIAnnotations := corev1.DownwardAPIVolumeFile{
		Path:     types.AnnotationsPath,
		FieldRef: &annotations,
	}
	dAPIItems := []corev1.DownwardAPIVolumeFile{dAPILabels, dAPIAnnotations}

	for _, hugepageResource := range hugepageResourceList {
		hugepageSelector := corev1.ResourceFieldSelector{
			Resource:      hugepageResource.ResourceName,
			ContainerName: hugepageResource.ContainerName,
			Divisor:       *resource.NewQuantity(1*1024*1024, resource.BinarySI),
		}
		dAPIHugepage := corev1.DownwardAPIVolumeFile{
			Path:             hugepageResource.Path,
			ResourceFieldRef: &hugepageSelector,
		}
		dAPIItems = append(dAPIItems, dAPIHugepage)
	}

	dAPIVolSource := corev1.DownwardAPIVolumeSource{
		Items: dAPIItems,
	}
	volSource := corev1.VolumeSource{
		DownwardAPI: &dAPIVolSource,
	}
	vol := corev1.Volume{
		Name:         "podnetinfo",
		VolumeSource: volSource,
	}

	patch = append(patch, jsonPatchOperation{
		Operation: "add",
		Path:      "/spec/volumes/-",
		Value:     vol,
	})

	return patch
}

func addVolumeMount(patch []jsonPatchOperation) []jsonPatchOperation {

	vm := corev1.VolumeMount{
		Name:      "podnetinfo",
		ReadOnly:  false,
		MountPath: types.DownwardAPIMountPath,
	}

	patch = append(patch, jsonPatchOperation{
		Operation: "add",
		Path:      "/spec/containers/0/volumeMounts/-", // NOTE: in future we may want to patch specific container (not always the first one)
		Value:     vm,
	})

	return patch
}

func createVolPatch(patch []jsonPatchOperation, hugepageResourceList []hugepageResourceData) []jsonPatchOperation {
	patch = addVolumeMount(patch)
	patch = addVolDownwardAPI(patch, hugepageResourceList)
	return patch
}

func addEnvVar(patch []jsonPatchOperation, containerIndex int, firstElement bool,
	envName string, envVal string) []jsonPatchOperation {

	env := corev1.EnvVar{
		Name:  envName,
		Value: envVal,
	}

	if firstElement {
		patch = append(patch, jsonPatchOperation{
			Operation: "add",
			Path:      "/spec/containers/" + fmt.Sprintf("%d", containerIndex) + "/env",
			Value:     []corev1.EnvVar{env},
		})
	} else {
		patch = append(patch, jsonPatchOperation{
			Operation: "add",
			Path:      "/spec/containers/" + fmt.Sprintf("%d", containerIndex) + "/env/-",
			Value:     env,
		})
	}

	return patch
}

func createEnvPatch(patch []jsonPatchOperation, container *corev1.Container,
	containerIndex int, envName string, envVal string) []jsonPatchOperation {

	// Determine if requested ENV already exists
	found := false
	firstElement := false
	if len(container.Env) != 0 {
		for _, env := range container.Env {
			if env.Name == envName {
				found = true
				if env.Value != envVal {
					glog.Warningf("Error, adding env '%s', name existed but value different: '%s' != '%s'",
						envName, env.Value, envVal)
				}
				break
			}
		}
	} else {
		firstElement = true
	}

	if !found {
		patch = addEnvVar(patch, containerIndex, firstElement, envName, envVal)
	}
	return patch
}

func createNodeSelectorPatch(patch []jsonPatchOperation, existing map[string]string, desired map[string]string) []jsonPatchOperation {
	targetMap := make(map[string]string)
	if existing != nil {
		for k, v := range existing {
			targetMap[k] = v
		}
	}
	if desired != nil {
		for k, v := range desired {
			targetMap[k] = v
		}
	}
	if len(targetMap) == 0 {
		return patch
	}
	patch = append(patch, jsonPatchOperation{
		Operation: "add",
		Path:      "/spec/nodeSelector",
		Value:     targetMap,
	})
	return patch
}

func createResourcePatch(patch []jsonPatchOperation, Containers []corev1.Container, resourceRequests map[string]int64) []jsonPatchOperation {
	/* check whether resources paths exists in the first container and add as the first patches if missing */
	if len(Containers[0].Resources.Requests) == 0 {
		patch = patchEmptyResources(patch, 0, "requests")
	}
	if len(Containers[0].Resources.Limits) == 0 {
		patch = patchEmptyResources(patch, 0, "limits")
	}

	resourceList := *getResourceList(resourceRequests)

	for resource, quantity := range resourceList {
		patch = appendResource(patch, resource.String(), quantity, quantity)
	}

	return patch
}

func updateResourcePatch(patch []jsonPatchOperation, Containers []corev1.Container, resourceRequests map[string]int64) []jsonPatchOperation {
	var existingrequestsMap map[corev1.ResourceName]resource.Quantity
	var existingLimitsMap map[corev1.ResourceName]resource.Quantity

	if len(Containers[0].Resources.Requests) == 0 {
		patch = patchEmptyResources(patch, 0, "requests")
	} else {
		existingrequestsMap = Containers[0].Resources.Requests
	}
	if len(Containers[0].Resources.Limits) == 0 {
		patch = patchEmptyResources(patch, 0, "limits")
	} else {
		existingLimitsMap = Containers[0].Resources.Limits
	}

	resourceList := *getResourceList(resourceRequests)

	for resourceName, quantity := range resourceList {
		reqQuantity := quantity
		limitQuantity := quantity
		if value, ok := existingrequestsMap[resourceName]; ok {
			reqQuantity.Add(value)
		}
		if value, ok := existingLimitsMap[resourceName]; ok {
			limitQuantity.Add(value)
		}
		patch = appendResource(patch, resourceName.String(), reqQuantity, limitQuantity)
	}

	return patch
}

func appendResource(patch []jsonPatchOperation, resourceName string, reqQuantity, limitQuantity resource.Quantity) []jsonPatchOperation {
	patch = append(patch, jsonPatchOperation{
		Operation: "add",
		Path:      "/spec/containers/0/resources/requests/" + toSafeJsonPatchKey(resourceName),
		Value:     reqQuantity,
	})
	patch = append(patch, jsonPatchOperation{
		Operation: "add",
		Path:      "/spec/containers/0/resources/limits/" + toSafeJsonPatchKey(resourceName),
		Value:     limitQuantity,
	})

	return patch
}

func getResourceList(resourceRequests map[string]int64) *corev1.ResourceList {
	resourceList := corev1.ResourceList{}
	for name, number := range resourceRequests {
		resourceList[corev1.ResourceName(name)] = *resource.NewQuantity(number, resource.DecimalSI)
	}

	return &resourceList
}

func createCustomizedPatch(pod corev1.Pod) ([]jsonPatchOperation, error) {
	var customizedPatch []jsonPatchOperation

	// lock for reading
	cusInjects.Lock()
	defer cusInjects.Unlock()

	for k, v := range cusInjects.Patchs {
		// The cusInjects will be injected when:
		// 1. Pod labels contain the patch key defined in cusInjects, and
		// 2. The value of patch key in pod labels(not in cusInjects) is "true"
		if podValue, exists := pod.ObjectMeta.Labels[k]; exists && strings.ToLower(podValue) == "true" {
			customizedPatch = append(customizedPatch, v)
		}
	}
	return customizedPatch, nil
}

func getNetworkSelections(annotationKey string, pod corev1.Pod, customizedPatch []jsonPatchOperation) (string, bool) {
	// User defined annotateKey takes precedence than customized injections
	glog.Infof("search %s in original pod annotations", annotationKey)
	nets, exists := pod.ObjectMeta.Annotations[annotationKey]
	if exists {
		glog.Infof("%s is defined in original pod annotations", annotationKey)
		return nets, exists
	}

	glog.Infof("search %s in customized injections", annotationKey)
	// customizedPatch may contain user defined net-attach-defs
	if len(customizedPatch) > 0 {
		for _, p := range customizedPatch {
			if p.Operation == "add" && p.Path == "/metadata/annotations" {
				for k, v := range p.Value.(map[string]interface{}) {
					if k == annotationKey {
						glog.Infof("%s is found in customized annotations", annotationKey)
						return v.(string), true
					}
				}
			}
		}
	}
	glog.Infof("%s is not found in either pod annotations or customized injections", annotationKey)
	return "", false
}

// MutateHandler handles AdmissionReview requests and sends responses back to the K8s API server
func MutateHandler(w http.ResponseWriter, req *http.Request) {
	glog.Infof("Received mutation request")
	var err error

	/* read AdmissionReview from the HTTP request */
	ar, httpStatus, err := readAdmissionReview(req, w)
	if err != nil {
		http.Error(w, err.Error(), httpStatus)
		return
	}

	/* read pod annotations */
	/* if networks missing skip everything */
	pod, err := deserializePod(ar)
	if err != nil {
		handleValidationError(w, ar, err)
		return
	}

	customizedPatch, err := createCustomizedPatch(pod)
	if err != nil {
		glog.Warningf("Error, failed to create customized injection patch, %v", err)
	}

	defaultNetSelection, defExist := getNetworkSelections(defaultNetworkAnnotationKey, pod, customizedPatch)
	additionalNetSelections, addExists := getNetworkSelections(networksAnnotationKey, pod, customizedPatch)

	if defExist || addExists {
		/* map of resources request needed by a pod and a number of them */
		resourceRequests := make(map[string]int64)

		/* map of node labels on which pod needs to be scheduled*/
		desiredNsMap := make(map[string]string)

		if defaultNetSelection != "" {
			defNetwork, err := parsePodNetworkSelections(defaultNetSelection, pod.ObjectMeta.Namespace)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if len(defNetwork) == 1 {
				resourceRequests, desiredNsMap, err = parseNetworkAttachDefinition(defNetwork[0], resourceRequests, desiredNsMap)
				if err != nil {
					err = prepareAdmissionReviewResponse(false, err.Error(), ar)
					if err != nil {
						glog.Errorf("error preparing AdmissionReview response: %s", err)
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					writeResponse(w, ar)
					return
				}
			}
		}
		if additionalNetSelections != "" {
			/* unmarshal list of network selection objects */
			networks, err := parsePodNetworkSelections(additionalNetSelections, pod.ObjectMeta.Namespace)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			for _, n := range networks {
				resourceRequests, desiredNsMap, err = parseNetworkAttachDefinition(n, resourceRequests, desiredNsMap)
				if err != nil {
					err = prepareAdmissionReviewResponse(false, err.Error(), ar)
					if err != nil {
						glog.Errorf("error preparing AdmissionReview response: %s", err)
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					writeResponse(w, ar)
					return
				}
			}
		}

		/* patch with custom resources requests and limits */
		err = prepareAdmissionReviewResponse(true, "allowed", ar)
		if err != nil {
			glog.Errorf("error preparing AdmissionReview response: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if len(resourceRequests) == 0 {
			glog.Infof("pod doesn't need any custom network resources")
		} else {
			var patch []jsonPatchOperation
			glog.Infof("honor-resources=%v", honorExistingResources)
			if honorExistingResources {
				patch = updateResourcePatch(patch, pod.Spec.Containers, resourceRequests)
			} else {
				patch = createResourcePatch(patch, pod.Spec.Containers, resourceRequests)
			}

			// Determine if hugepages are being requested for a given container,
			// and if so, expose the value to the container via Downward API.
			var hugepageResourceList []hugepageResourceData
			glog.Infof("injectHugepageDownApi=%v", injectHugepageDownApi)
			if injectHugepageDownApi {
				for containerIndex, container := range pod.Spec.Containers {
					found := false
					if len(container.Resources.Requests) != 0 {
						if quantity, exists := container.Resources.Requests["hugepages-1Gi"]; exists && quantity.IsZero() == false {
							hugepageResource := hugepageResourceData{
								ResourceName:  "requests.hugepages-1Gi",
								ContainerName: container.Name,
								Path:          types.Hugepages1GRequestPath + "_" + container.Name,
							}
							hugepageResourceList = append(hugepageResourceList, hugepageResource)
							found = true
						}
						if quantity, exists := container.Resources.Requests["hugepages-2Mi"]; exists && quantity.IsZero() == false {
							hugepageResource := hugepageResourceData{
								ResourceName:  "requests.hugepages-2Mi",
								ContainerName: container.Name,
								Path:          types.Hugepages2MRequestPath + "_" + container.Name,
							}
							hugepageResourceList = append(hugepageResourceList, hugepageResource)
							found = true
						}
					}
					if len(container.Resources.Limits) != 0 {
						if quantity, exists := container.Resources.Limits["hugepages-1Gi"]; exists && quantity.IsZero() == false {
							hugepageResource := hugepageResourceData{
								ResourceName:  "limits.hugepages-1Gi",
								ContainerName: container.Name,
								Path:          types.Hugepages1GLimitPath + "_" + container.Name,
							}
							hugepageResourceList = append(hugepageResourceList, hugepageResource)
							found = true
						}
						if quantity, exists := container.Resources.Limits["hugepages-2Mi"]; exists && quantity.IsZero() == false {
							hugepageResource := hugepageResourceData{
								ResourceName:  "limits.hugepages-2Mi",
								ContainerName: container.Name,
								Path:          types.Hugepages2MLimitPath + "_" + container.Name,
							}
							hugepageResourceList = append(hugepageResourceList, hugepageResource)
							found = true
						}
					}

					// If Hugepages are being added to Downward API, add the
					// 'container.Name' as an environment variable to the container
					// so container knows its name and can process hugepages properly.
					if found {
						patch = createEnvPatch(patch, &container, containerIndex,
							types.EnvNameContainerName, container.Name)
					}
				}
			}

			patch = createNodeSelectorPatch(patch, pod.Spec.NodeSelector, desiredNsMap)
			patch = createVolPatch(patch, hugepageResourceList)
			patch = append(patch, customizedPatch...)
			glog.Infof("patch after all mutations: %v", patch)

			patchBytes, _ := json.Marshal(patch)
			ar.Response.Patch = patchBytes
			ar.Response.PatchType = func() *v1beta1.PatchType {
				pt := v1beta1.PatchTypeJSONPatch
				return &pt
			}()
		}
	} else {
		/* network annotation not provided or empty */
		glog.Infof("pod spec doesn't have network annotations. Skipping...")
		err = prepareAdmissionReviewResponse(true, "Pod spec doesn't have network annotations. Skipping...", ar)
		if err != nil {
			glog.Infof("error preparing AdmissionReview response: %s", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	writeResponse(w, ar)
	return

}

// SetResourceNameKeys extracts resources from a string and add them to resourceNameKeys array
func SetResourceNameKeys(keys string) error {
	if keys == "" {
		return errors.New("resoure keys can not be empty")
	}
	for _, resourceNameKey := range strings.Split(keys, ",") {
		resourceNameKey = strings.TrimSpace(resourceNameKey)
		resourceNameKeys = append(resourceNameKeys, resourceNameKey)
	}
	return nil
}

// SetupInClusterClient setups K8s client to communicate with the API server
func SetupInClusterClient() kubernetes.Interface {
	/* setup Kubernetes API client */
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatal(err)
	}
	return clientset
}

// SetInjectHugepageDownApi sets a flag to indicate whether or not to inject the
// hugepage request and limit for the Downward API.
func SetInjectHugepageDownApi(hugepageFlag bool) {
	injectHugepageDownApi = hugepageFlag
}

// SetHonorExistingResources initialize the honorExistingResources flag
func SetHonorExistingResources(resourcesHonorFlag bool) {
	honorExistingResources = resourcesHonorFlag
}

// SetCustomizedInjections sets additional injections to be applied in Pod spec
func SetCustomizedInjections(injections *corev1.ConfigMap) {
	// lock for writing
	cusInjects.Lock()
	defer cusInjects.Unlock()

	var patch jsonPatchOperation
	var cusPatchs = cusInjects.Patchs

	for k, v := range injections.Data {
		existValue, exists := cusPatchs[k]
		// unmarshall customized injection to json patch
		err := json.Unmarshal([]byte(v), &patch)
		if err != nil {
			glog.Errorf("Failed to unmarshall customized injection: %v", v)
			continue
		}
		// metadata.Annotations is the only supported field for customization
		// jsonPatchOperation.Path should be "/metadata/annotations" when customizing pod annotation
		if patch.Path != "/metadata/annotations" {
			glog.Errorf("Path: %v is not supported, only /metadata/annotations can be customized", patch.Path)
			continue
		}
		if !exists || !reflect.DeepEqual(existValue, patch) {
			glog.Infof("Initializing customized injections with key: %v, value: %v", k, v)
			cusPatchs[k] = patch
		}
	}
	// remove stale entries from customized configMap
	for k, _ := range cusPatchs {
		if _, ok := injections.Data[k]; ok {
			continue
		}
		glog.Infof("Removing stale entry: %v from customized injections", k)
		delete(cusPatchs, k)
	}
}
