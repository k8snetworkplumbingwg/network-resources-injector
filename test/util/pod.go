package util

import (
	"context"
	"errors"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
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
func GetPodDefinition(ns string) *corev1.Pod {
	var graceTime int64 = 0
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nri-e2e-test",
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

//GetOneNetwork add one network to pod
func GetOneNetwork(nad, ns string) *corev1.Pod {
	pod := GetPodDefinition(ns)
	pod.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": nad}
	return pod
}

//GetMultiNetworks adds a network to annotation
func GetMultiNetworks(nad []string, ns string) *corev1.Pod {
	pod := GetPodDefinition(ns)
	pod.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": strings.Join(nad, ",")}
	return pod
}

//WaitForPodStateRunning waits for pod to enter running state
func WaitForPodStateRunning(core coreclient.CoreV1Interface, podName, ns string, timeout, interval time.Duration) error {
	time.Sleep(30 * time.Second)
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
