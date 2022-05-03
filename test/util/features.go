package util

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreclient "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	hugepagesResourceName2Mi = "hugepages-2Mi"
	hugepagesResouceName1Gi  = "hugepages-1Gi"
)

//IsMinHugepagesAvailable checks if a min Gi/Mi hugepage number is available on any nodes
func IsMinHugepagesAvailable(ci coreclient.CoreV1Interface, minGi, minMi int) (bool, error) {
	hugepages := map[string]int{hugepagesResouceName1Gi: minGi, hugepagesResourceName2Mi: minMi}
	list, err := ci.Nodes().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}

	if minGi == 0 && minMi == 0 {
		return false, fmt.Errorf("IsHugepagesAvailable(): define at least greater than zero hugepages")
	}

	if len(list.Items) == 0 {
		return false, fmt.Errorf("IsHugepagesAvailable(): no nodes available")
	}

	foundOnce := map[string]bool{hugepagesResouceName1Gi: false, hugepagesResourceName2Mi: false}

	for _, node := range list.Items {
		for resource, minHugepages := range hugepages {
			if minHugepages == 0 {
				foundOnce[resource] = true
				continue
			}
			capacity, ok := node.Status.Capacity[v1.ResourceName(resource)]
			if !ok {
				return false, fmt.Errorf("IsHugepagesAvailable(): cannot find hugepage resource %s via K8 API", resource)
			}
			size, ok := capacity.AsInt64()
			if !ok {
				return false, fmt.Errorf("IsHugepagesAvailable(): failed to convert hugepage capacity")
			}
			if int64(minHugepages) <= size {
				foundOnce[resource] = true
			}
		}
	}
	return foundOnce[hugepagesResourceName2Mi] && foundOnce[hugepagesResouceName1Gi], nil
}
