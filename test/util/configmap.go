package util

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
)

func GetConfigMap(confgiMapName, namespace string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      confgiMapName,
			Namespace: namespace,
		},
	}
}

func AddData(configMap *corev1.ConfigMap, dataKey, dataValue string) *corev1.ConfigMap {
	if nil == configMap.Data {
		configMap.Data = make(map[string]string)
	}

	configMap.Data[dataKey] = dataValue

	return configMap
}

func ApplyConfigMap(ci coreclient.CoreV1Interface, configMap *corev1.ConfigMap, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	defer cancel()
	_, err := ci.ConfigMaps(configMap.Namespace).Create(ctx, configMap, metav1.CreateOptions{})

	if err != nil {
		return err
	}

	return nil
}

func DeleteConfigMap(ci coreclient.CoreV1Interface, configMap *corev1.ConfigMap, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	err := ci.ConfigMaps(configMap.Namespace).Delete(ctx, configMap.Name, metav1.DeleteOptions{})
	return err
}
