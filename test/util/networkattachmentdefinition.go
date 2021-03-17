package util

import (
	"context"
	"time"

	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	networkCoreClient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetWithoutAnnotations(ns string, networkName string) *cniv1.NetworkAttachmentDefinition {
	nad := GetNetworkAttachmentDefinition(ns, networkName)

	return nad
}

func GetResourceSelectorOnly(ns string, networkName string, resourceName string) *cniv1.NetworkAttachmentDefinition {
	nad := GetNetworkAttachmentDefinition(ns, networkName)
	nad.Annotations = map[string]string{"k8s.v1.cni.cncf.io/resourceName": resourceName}

	return nad
}

func GetNodeSelectorOnly(ns string, networkName string, nodeName string) *cniv1.NetworkAttachmentDefinition {
	nad := GetNetworkAttachmentDefinition(ns, networkName)
	nad.Annotations = map[string]string{"k8s.v1.cni.cncf.io/nodeSelector": nodeName}

	return nad
}

func GetResourceAndNodeSelector(ns string, networkName string, nodeName string) *cniv1.NetworkAttachmentDefinition {
	nad := GetNetworkAttachmentDefinition(ns, networkName)
	nad.Annotations = map[string]string{
		"k8s.v1.cni.cncf.io/nodeSelector": nodeName,
		"k8s.v1.cni.cncf.io/resourceName": "example.com/foo",
	}

	return nad
}

func GetNetworkAttachmentDefinition(networkName string, ns string) *cniv1.NetworkAttachmentDefinition {
	return &cniv1.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      networkName,
			Namespace: ns,
		},
		Spec: cniv1.NetworkAttachmentDefinitionSpec{
			Config: GetNetworkSpecConfig(networkName),
		},
	}
}

// {
// 				"cniVersion": "0.3.0",
// 				"name":       networkName,
// 				"type":       "loopback",
// 			},
func GetNetworkSpecConfig(networkName string) string {
	config := "{\"cniVersion\": \"0.3.0\", \"name\": \"" + networkName + "\", \"type\":\"loopback\"}"
	return config
}

func ApplyNetworkAttachmentDefinition(ci networkCoreClient.K8sCniCncfIoV1Interface, nad *cniv1.NetworkAttachmentDefinition, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	defer cancel()
	_, err := ci.NetworkAttachmentDefinitions(nad.Namespace).Create(ctx, nad, metav1.CreateOptions{})

	if err != nil {
		return err
	}

	return nil
}

func DeleteNetworkAttachmentDefinition(ci networkCoreClient.K8sCniCncfIoV1Interface, testNetworkName string, nad *cniv1.NetworkAttachmentDefinition, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	defer cancel()
	err := ci.NetworkAttachmentDefinitions(nad.Namespace).Delete(ctx, testNetworkName, metav1.DeleteOptions{})

	if err != nil {
		return err
	}

	return nil
}
