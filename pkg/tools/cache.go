// Copyright (c) 2021 Nordix Foundation.
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

package cache

import (
	"sync"

	"github.com/golang/glog"
	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	"github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type NetAttachDefCache struct {
	networkAnnotationsMap      map[string]map[string]string
	networkAnnotationsMapMutex *sync.Mutex
	stopper                    chan struct{}
}

type NetAttachDefCacheService interface {
	Start()
	Stop()
	Get(string) map[string]string
}

func Create() NetAttachDefCacheService {
	return &NetAttachDefCache{make(map[string]map[string]string),
		&sync.Mutex{}, make(chan struct{})}
}

// Start start the informer for NetworkAttachmentDefinition, upon events populate the cache
func (nc *NetAttachDefCache) Start() {
	factory := externalversions.NewFilteredSharedInformerFactory(setupNetAttachDefClient(), 0, "", func(o *metav1.ListOptions) {})
	informer := factory.K8sCniCncfIo().V1().NetworkAttachmentDefinitions().Informer()
	// mutex to serialize the events.
	mutex := &sync.Mutex{}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mutex.Lock()
			defer mutex.Unlock()
			netAttachDef := obj.(*cniv1.NetworkAttachmentDefinition)
			nc.put(netAttachDef.Namespace+"/"+netAttachDef.Name, netAttachDef.Annotations)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			mutex.Lock()
			defer mutex.Unlock()
			oldNetAttachDef := oldObj.(*cniv1.NetworkAttachmentDefinition)
			nc.remove(oldNetAttachDef.Namespace + "/" + oldNetAttachDef.Name)
			newNetAttachDef := newObj.(*cniv1.NetworkAttachmentDefinition)
			nc.put(newNetAttachDef.Namespace+"/"+newNetAttachDef.Name, newNetAttachDef.Annotations)
		},
		DeleteFunc: func(obj interface{}) {
			mutex.Lock()
			defer mutex.Unlock()
			netAttachDef := obj.(*cniv1.NetworkAttachmentDefinition)
			nc.remove(netAttachDef.Namespace + "/" + netAttachDef.Name)
		},
	})
	go informer.Run(nc.stopper)
}

// Stop stop the NetworkAttachmentDefinition informer
func (nc *NetAttachDefCache) Stop() {
	close(nc.stopper)
	nc.networkAnnotationsMapMutex.Lock()
	nc.networkAnnotationsMap = nil
	nc.networkAnnotationsMapMutex.Unlock()
}

func (nc *NetAttachDefCache) put(networkName string, annotations map[string]string) {
	nc.networkAnnotationsMapMutex.Lock()
	nc.networkAnnotationsMap[networkName] = annotations
	nc.networkAnnotationsMapMutex.Unlock()
}

// Get returns annotations map for the given network name, if it's not available
// return nil
func (nc *NetAttachDefCache) Get(networkName string) map[string]string {
	nc.networkAnnotationsMapMutex.Lock()
	defer nc.networkAnnotationsMapMutex.Unlock()
	if annotationsMap, exists := nc.networkAnnotationsMap[networkName]; exists {
		return annotationsMap
	}
	return nil
}

func (nc *NetAttachDefCache) remove(networkName string) {
	nc.networkAnnotationsMapMutex.Lock()
	delete(nc.networkAnnotationsMap[networkName], networkName)
	nc.networkAnnotationsMapMutex.Unlock()
}

// setupNetAttachDefClient creates K8s client for net-attach-def crd
func setupNetAttachDefClient() versioned.Interface {
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatal(err)
	}
	clientset, err := versioned.NewForConfig(config)
	if err != nil {
		glog.Fatal(err)
	}
	return clientset
}
