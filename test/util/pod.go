package util

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

//CreateRunningPod create a pod and wait until it is running
func CreateRunningPod(ci coreclient.CoreV1Interface, pod *corev1.Pod, timeout, interval time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	pod, err := ci.Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})

	if err != nil {
		return err
	}

	err = WaitForPodStateRunning(ci, pod.ObjectMeta.Name, pod.ObjectMeta.Namespace, timeout, interval)
	if err != nil {
		return err
	}

	return nil
}

//DeletePod will delete a pod
func DeletePod(ci coreclient.CoreV1Interface, pod *corev1.Pod, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := ci.Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	return err
}

//UpdatePodInfo will get the current pod state and return it
func UpdatePodInfo(ci coreclient.CoreV1Interface, pod *corev1.Pod, timeout time.Duration) (*corev1.Pod, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	pod, err := ci.Pods(pod.Namespace).Get(ctx, pod.ObjectMeta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return pod, nil
}

//GetPodDefinition will return a test pod
func GetPodDefinition(ns string, podName string) *corev1.Pod {
	var graceTime int64 = 0
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: &graceTime,
			Containers: []corev1.Container{
				{
					Name:    "test",
					Image:   GetPodTestImage(),
					Command: []string{"/bin/sh", "-c", "sleep INF"},
				},
			},
		},
	}
}

//AddMetadataLabel adds label to the POD metadata section
// :param labelName - key in map, label name
// :param labelContent - value, label content
func AddMetadataLabel(pod *corev1.Pod, labelName, labelContent string) *corev1.Pod {
	if nil == pod.ObjectMeta.Labels {
		pod.ObjectMeta.Labels = make(map[string]string)
	}

	pod.ObjectMeta.Labels[labelName] = labelContent

	return pod
}

//AddToPodDefinitionVolumesWithDownwardAPI adds to the POD specification at the 'path' downwardAPI volumes that expose POD namespace
// :param pod - POD object to be modified
// :param mountPath - path of the folder in which file is going to be available
// :param volumeName - name of the volume
// :param containerNumber - number of the container to which volumes have to be added
// :return updated POD object
func AddToPodDefinitionVolumesWithDownwardAPI(pod *corev1.Pod, mountPath, volumeName string, containerNumber int64) *corev1.Pod {
	pod.Spec.Volumes = []corev1.Volume{
		{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				DownwardAPI: &corev1.DownwardAPIVolumeSource{
					Items: []corev1.DownwardAPIVolumeFile{
						{
							Path: "namespace",
							FieldRef: &corev1.ObjectFieldSelector{
								FieldPath: "metadata.namespace",
							},
						},
					},
				},
			},
		},
	}

	pod.Spec.Containers[containerNumber].VolumeMounts = []corev1.VolumeMount{
		{
			Name:      volumeName,
			ReadOnly:  false,
			MountPath: mountPath,
		},
	}

	return pod
}

// AddToPodDefinitionHugePages1Gi adds Hugepages 1Gi limits and requirements to the POD spec
func AddToPodDefinitionHugePages1Gi(pod *corev1.Pod, amountLimit, amountRequest, containerNumber int64) *corev1.Pod {
	if nil == pod.Spec.Containers[containerNumber].Resources.Limits {
		pod.Spec.Containers[containerNumber].Resources.Limits = make(map[corev1.ResourceName]resource.Quantity)
	}

	if nil == pod.Spec.Containers[containerNumber].Resources.Requests {
		pod.Spec.Containers[containerNumber].Resources.Requests = make(map[corev1.ResourceName]resource.Quantity)
	}

	pod.Spec.Containers[containerNumber].Resources.Limits["hugepages-1Gi"] = *resource.NewQuantity(amountLimit*1024*1024*1024, resource.BinarySI)
	pod.Spec.Containers[containerNumber].Resources.Requests["hugepages-1Gi"] = *resource.NewQuantity(amountRequest*1024*1024*1024, resource.BinarySI)

	return pod
}

// AddToPodDefinitionHugePages2Mi adds Hugepages 2Mi limits and requirements to the POD spec
func AddToPodDefinitionHugePages2Mi(pod *corev1.Pod, amountLimit, amountRequest, containerNumber int64) *corev1.Pod {
	if nil == pod.Spec.Containers[containerNumber].Resources.Limits {
		pod.Spec.Containers[containerNumber].Resources.Limits = make(map[corev1.ResourceName]resource.Quantity)
	}

	if nil == pod.Spec.Containers[containerNumber].Resources.Requests {
		pod.Spec.Containers[containerNumber].Resources.Requests = make(map[corev1.ResourceName]resource.Quantity)
	}

	pod.Spec.Containers[containerNumber].Resources.Limits["hugepages-2Mi"] = *resource.NewQuantity(amountLimit*1024*1024, resource.BinarySI)
	pod.Spec.Containers[containerNumber].Resources.Requests["hugepages-2Mi"] = *resource.NewQuantity(amountRequest*1024*1024, resource.BinarySI)

	return pod
}

// AddToPodDefinitionMemory adds Memory constraints to the POD spec
func AddToPodDefinitionMemory(pod *corev1.Pod, amountLimit, amountRequest, containerNumber int64) *corev1.Pod {
	if nil == pod.Spec.Containers[containerNumber].Resources.Limits {
		pod.Spec.Containers[containerNumber].Resources.Limits = make(map[corev1.ResourceName]resource.Quantity)
	}

	if nil == pod.Spec.Containers[containerNumber].Resources.Requests {
		pod.Spec.Containers[containerNumber].Resources.Requests = make(map[corev1.ResourceName]resource.Quantity)
	}

	pod.Spec.Containers[containerNumber].Resources.Limits["memory"] = *resource.NewQuantity(amountLimit*1024*1024*1024, resource.BinarySI)
	pod.Spec.Containers[containerNumber].Resources.Requests["memory"] = *resource.NewQuantity(amountRequest*1024*1024*1024, resource.BinarySI)

	return pod
}

// AddToPodDefinitionCpuLimits adds CPU limits and requests to the definition of POD
func AddToPodDefinitionCpuLimits(pod *corev1.Pod, cpuNumber, containerNumber int64) *corev1.Pod {
	if nil == pod.Spec.Containers[containerNumber].Resources.Limits {
		pod.Spec.Containers[containerNumber].Resources.Limits = make(map[corev1.ResourceName]resource.Quantity)
	}

	if nil == pod.Spec.Containers[containerNumber].Resources.Requests {
		pod.Spec.Containers[containerNumber].Resources.Requests = make(map[corev1.ResourceName]resource.Quantity)
	}

	pod.Spec.Containers[containerNumber].Resources.Limits["cpu"] = *resource.NewQuantity(cpuNumber, resource.DecimalSI)
	pod.Spec.Containers[containerNumber].Resources.Requests["cpu"] = *resource.NewQuantity(cpuNumber, resource.DecimalSI)

	return pod
}

//GetOneNetwork add one network to pod
func GetOneNetwork(nad, ns string, podName string) *corev1.Pod {
	pod := GetPodDefinition(ns, podName)
	pod.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": nad}
	return pod
}

//GetOneNetworkTwoContainers returns POD with two containers and one network
func GetOneNetworkTwoContainers(nad, ns, podName, secondContainerName string) *corev1.Pod {
	pod := GetPodDefinition(ns, podName)
	pod.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": nad}

	pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{
		Name:    secondContainerName,
		Image:   GetPodTestImage(),
		Command: []string{"/bin/sh", "-c", "sleep INF"},
	})

	return pod
}

//GetMultiNetworks adds a network to annotation
func GetMultiNetworks(nad []string, ns string, podName string) *corev1.Pod {
	pod := GetPodDefinition(ns, podName)
	pod.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": strings.Join(nad, ",")}
	return pod
}

//WaitForPodStateRunning waits for pod to enter running state
func WaitForPodStateRunning(core coreclient.CoreV1Interface, podName, ns string, timeout, interval time.Duration) error {
	return wait.PollImmediate(interval, timeout, func() (done bool, err error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		pod, err := core.Pods(ns).Get(ctx, podName, metav1.GetOptions{})

		if err != nil {
			return false, err
		}
		switch pod.Status.Phase {
		case corev1.PodRunning:
			return true, nil
		case corev1.PodFailed, corev1.PodSucceeded:
			return false, errors.New("pod failed or succeeded but is not running")
		}
		return false, nil
	})
}

// ExecuteCommand execute command on the POD
// :param core - core V1 Interface
// :param config - configuration used to establish REST connection with K8s node
// :param podName - POD name on which command has to be executed
// :param ns - namespace in which POD exists
// :param containerName - container name on which command should be executed
// :param command - command to be executed on POD
// :return string output of the command (stdout)
// 	       string output of the command (stderr)
//         error Error object or when everthing is correct nil
func ExecuteCommand(core coreclient.CoreV1Interface, config *restclient.Config, podName, ns, containerName, command string) (string, string, error) {
	shellCommand := []string{"/bin/sh", "-c", command}
	request := core.RESTClient().Post().Resource("pods").Name(podName).Namespace(ns).SubResource("exec")
	options := &corev1.PodExecOptions{
		Command:   shellCommand,
		Container: containerName,
		Stdin:     false,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}

	request.VersionedParams(options, scheme.ParameterCodec)
	exec, err := remotecommand.NewSPDYExecutor(config, "POST", request.URL())
	if nil != err {
		return "", "", err
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  nil,
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	if nil != err {
		return "", "", err
	}

	return stdout.String(), stderr.String(), nil
}
