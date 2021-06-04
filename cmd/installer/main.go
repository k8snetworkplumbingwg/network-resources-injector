// Copyright (c) 2019 Intel Corporation
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

package main

import (
	"flag"

	"github.com/golang/glog"
	"github.com/k8snetworkplumbingwg/network-resources-injector/pkg/installer"
	"github.com/k8snetworkplumbingwg/network-resources-injector/pkg/types"
)

func main() {
	namespace := flag.String("namespace", "kube-system",
		"Namespace in which all Kubernetes resources will be created.")
	prefix := flag.String("name", "network-resources-injector",
		"Prefix added to the names of all created resources.")
	webhookPort := flag.Int("webhook-port", types.DefaultWebhookPort, "Port number which webhook will serve")
	webhookSvcPort := flag.Int("webhook-service-port", types.DefaultServicePort, "Port number for webhook service")

	if *webhookPort < 1024 || *webhookPort > 65535 {
		glog.Fatalf("invalid port number. Choose between 1024 and 65535")
	}

	flag.Parse()

	glog.Info("starting webhook installation")
	installer.Install(*namespace, *prefix, *webhookPort, *webhookSvcPort)
}
