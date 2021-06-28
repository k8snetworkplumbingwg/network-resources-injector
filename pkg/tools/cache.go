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
	"sync/atomic"
	"time"

	"github.com/golang/glog"
	cniv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	"github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/informers/externalversions"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

type NetAttachDefCache struct {
	networkAnnotationsMap      map[string]map[string]string
	networkAnnotationsMapMutex *sync.Mutex
	stopper                    chan struct{}
	isRunning                  int32
}

type NetAttachDefCacheService interface {
	Start()
	Stop()
	Get(namespace string, networkName string) map[string]string
}

func Create() NetAttachDefCacheService {
	return &NetAttachDefCache{make(map[string]map[string]string),
		&sync.Mutex{}, make(chan struct{}), 0}
}

// Start creates informer for NetworkAttachmentDefinition events and populate the local cache
func (nc *NetAttachDefCache) Start() {
	factory := externalversions.NewSharedInformerFactoryWithOptions(setupNetAttachDefClient(), 0, externalversions.WithNamespace(""))
	informer := factory.K8sCniCncfIo().V1().NetworkAttachmentDefinitions().Informer()
	// mutex to serialize the events.
	mutex := &sync.Mutex{}

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			mutex.Lock()
			defer mutex.Unlock()
			netAttachDef := obj.(*cniv1.NetworkAttachmentDefinition)
			nc.put(netAttachDef.Namespace, netAttachDef.Name, netAttachDef.Annotations)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			mutex.Lock()
			defer mutex.Unlock()
			oldNetAttachDef := oldObj.(*cniv1.NetworkAttachmentDefinition)
			newNetAttachDef := newObj.(*cniv1.NetworkAttachmentDefinition)
			if oldNetAttachDef.GetResourceVersion() == newNetAttachDef.GetResourceVersion() {
				glog.Infof("no change in net-attach-def %s, ignoring update event", nc.getKey(oldNetAttachDef.Namespace, newNetAttachDef.Name))
				return
			}
			nc.remove(oldNetAttachDef.Namespace, oldNetAttachDef.Name)
			nc.put(newNetAttachDef.Namespace, newNetAttachDef.Name, newNetAttachDef.Annotations)
		},
		DeleteFunc: func(obj interface{}) {
			mutex.Lock()
			defer mutex.Unlock()
			netAttachDef := obj.(*cniv1.NetworkAttachmentDefinition)
			nc.remove(netAttachDef.Namespace, netAttachDef.Name)
		},
	})
	go func() {
		atomic.StoreInt32(&(nc.isRunning), int32(1))
		// informer Run blocks until informer is stopped
		glog.Infof("starting net-attach-def informer")
		informer.Run(nc.stopper)
		glog.Infof("net-attach-def informer is stopped")
		atomic.StoreInt32(&(nc.isRunning), int32(0))
	}()
}

// Stop teardown the NetworkAttachmentDefinition informer
func (nc *NetAttachDefCache) Stop() {
	close(nc.stopper)
	tEnd := time.Now().Add(3 * time.Second)
	for tEnd.After(time.Now()) {
		if atomic.LoadInt32(&nc.isRunning) == 0 {
			glog.Infof("net-attach-def informer is no longer running, proceed to clean up nad cache")
			break
		}
		time.Sleep(600 * time.Millisecond)
	}
	nc.networkAnnotationsMapMutex.Lock()
	nc.networkAnnotationsMap = nil
	nc.networkAnnotationsMapMutex.Unlock()
}

func (nc *NetAttachDefCache) put(namespace, networkName string, annotations map[string]string) {
	nc.networkAnnotationsMapMutex.Lock()
	nc.networkAnnotationsMap[nc.getKey(namespace, networkName)] = annotations
	nc.networkAnnotationsMapMutex.Unlock()
}

// Get returns annotations map for the given namespace and network name, if it's not available
// return nil
func (nc *NetAttachDefCache) Get(namespace, networkName string) map[string]string {
	nc.networkAnnotationsMapMutex.Lock()
	defer nc.networkAnnotationsMapMutex.Unlock()
	if annotationsMap, exists := nc.networkAnnotationsMap[nc.getKey(namespace, networkName)]; exists {
		return annotationsMap
	}
	return nil
}

func (nc *NetAttachDefCache) remove(namespace, networkName string) {
	nc.networkAnnotationsMapMutex.Lock()
	delete(nc.networkAnnotationsMap, nc.getKey(namespace, networkName))
	nc.networkAnnotationsMapMutex.Unlock()
}

func (nc *NetAttachDefCache) getKey(namespace, networkName string) string {
	return namespace + "/" + networkName
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
