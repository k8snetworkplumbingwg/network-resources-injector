package util

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
)

func CreateNamespace(ci coreclient.CoreV1Interface, name string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	nsSpec := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err := ci.Namespaces().Create(ctx, nsSpec, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func DeleteNamespace(ci coreclient.CoreV1Interface, name string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	defer cancel()

	err := ci.Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err == nil {
		WaitForNamespaceDelete(ci, name, timeout, 10)
	}

	return err
}

func WaitForNamespaceDelete(core coreclient.CoreV1Interface, namespaceName string, timeout, interval time.Duration) error {
	return wait.PollImmediate(interval, timeout, func() (done bool, err error) {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		namespace, err := core.Namespaces().Get(ctx, namespaceName, metav1.GetOptions{})

		if err != nil {
			return false, err
		}

		switch namespace.Status.Phase {
		case corev1.NamespaceActive, corev1.NamespaceTerminating:
			return false, nil
		default:
			return true, nil
		}

		return false, nil
	})
}
