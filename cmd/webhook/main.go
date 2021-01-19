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
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/k8snetworkplumbingwg/network-resources-injector/pkg/webhook"
)

const (
	defaultClientCa = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	readTo          = 5 * time.Second
	writeTo         = 10 * time.Second
	readHeaderTo    = 1 * time.Second
	serviceTo       = 2 * time.Second
)

func main() {
	var clientCAPaths webhook.ClientCAFlags
	/* load configuration */
	port := flag.Int("port", 8443, "The port on which to serve.")
	address := flag.String("bind-address", "0.0.0.0", "The IP address on which to listen for the --port port.")
	cert := flag.String("tls-cert-file", "cert.pem", "File containing the default x509 Certificate for HTTPS.")
	key := flag.String("tls-private-key-file", "key.pem", "File containing the default x509 private key matching --tls-cert-file.")
	insecure := flag.Bool("insecure", false, "Disable adding client CA to server TLS endpoint --insecure")
	injectHugepageDownApi := flag.Bool("injectHugepageDownApi", false, "Enable hugepage requests and limits into Downward API.")
	flag.Var(&clientCAPaths, "client-ca", "File containing client CA. This flag is repeatable if more than one client CA needs to be added to server")
	resourceNameKeys := flag.String("network-resource-name-keys", "k8s.v1.cni.cncf.io/resourceName", "comma separated resource name keys --network-resource-name-keys.")
	resourcesHonorFlag := flag.Bool("honor-resources", false, "Honor the existing requested resources requests & limits --honor-resources")
	flag.Parse()

	if *port < 1024 || *port > 65535 {
		glog.Fatalf("invalid port number. Choose between 1024 and 65535")
	}

	if *address == "" || *cert == "" || *key == "" || *resourceNameKeys == "" {
		glog.Fatalf("input argument(s) not defined correctly")
	}

	if len(clientCAPaths) == 0 {
		clientCAPaths = append(clientCAPaths, defaultClientCa)
	}

	glog.Infof("starting mutating admission controller for network resources injection")

	keyPair, err := webhook.NewTlsKeyPairReloader(*cert, *key)
	if err != nil {
		glog.Fatalf("error load certificate: %s", err.Error())
	}

	clientCaPool, err := webhook.NewClientCertPool(&clientCAPaths, *insecure)
	if err != nil {
		glog.Fatalf("error loading client CA pool: '%s'", err.Error())
	}

	/* init API client */
	webhook.SetupInClusterClient()

	webhook.SetInjectHugepageDownApi(*injectHugepageDownApi)

	webhook.SetHonorExistingResources(*resourcesHonorFlag)

	err = webhook.SetResourceNameKeys(*resourceNameKeys)
	if err != nil {
		glog.Fatalf("error in setting resource name keys: %s", err.Error())
	}

	watcher := webhook.NewKeyPairWatcher(keyPair, serviceTo)
	if err = watcher.Run(); err != nil {
		glog.Fatalf("starting TLS key & cert file watcher failed: '%s'", err.Error())
	}

	server := webhook.NewMutateServer(*address, *port, *insecure, readTo, writeTo, readHeaderTo, serviceTo, clientCaPool, keyPair)
	if err = server.Run(); err != nil {
		watcher.Quit()
		glog.Fatalf("starting HTTP server failed: '%s'", err)
	}

	/* Blocks until termination or TLS key/cert file watcher or HTTP server signal occurs and stops HTTP server/file watcher */
	if err := webhook.Watch(server, watcher, make(chan os.Signal, 1)); err != nil {
		glog.Error(err.Error())
	}
}
