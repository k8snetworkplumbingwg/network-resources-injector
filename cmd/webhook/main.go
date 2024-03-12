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

//test comment

package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/k8snetworkplumbingwg/network-resources-injector/pkg/controlswitches"
	netcache "github.com/k8snetworkplumbingwg/network-resources-injector/pkg/tools"
	"github.com/k8snetworkplumbingwg/network-resources-injector/pkg/userdefinedinjections"
	"github.com/k8snetworkplumbingwg/network-resources-injector/pkg/webhook"
)

const (
	defaultClientCa          = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	controlSwitchesConfigMap = "nri-control-switches"
)

func main() {
	var namespace string
	var clientCAPaths webhook.ClientCAFlags

	/* load configuration */
	port := flag.Int("port", 8443, "The port on which to serve.")
	address := flag.String("bind-address", "0.0.0.0", "The IP address on which to listen for the --port port.")
	cert := flag.String("tls-cert-file", "cert.pem", "File containing the default x509 Certificate for HTTPS.")
	key := flag.String("tls-private-key-file", "key.pem", "File containing the default x509 private key matching --tls-cert-file.")
	insecure := flag.Bool("insecure", false, "Disable adding client CA to server TLS endpoint --insecure")
	flag.Var(&clientCAPaths, "client-ca", "File containing client CA. This flag is repeatable if more than one client CA needs to be added to server")
	healthCheckPort := flag.Int("health-check-port", 8444, "The port to use for health check monitoring")
	enableHTTP2 := flag.Bool("enable-http2", false, "If HTTP/2 should be enabled for the webhook server.")

	// do initialization of control switches flags
	controlSwitches := controlswitches.SetupControlSwitchesFlags()

	// at the end when all flags are declared parse it
	flag.Parse()

	// initialize all control switches structures
	controlSwitches.InitControlSwitches()
	glog.Infof("controlSwitches: %+v", *controlSwitches)

	if !isValidPort(*port) {
		glog.Fatalf("invalid port number. Choose between 1024 and 65535")
	}

	if !controlSwitches.IsResourcesNameEnabled() {
		glog.Fatalf("Input argument for resourceName cannot be empty.")
	}

	if *address == "" || *cert == "" || *key == "" {
		glog.Fatalf("input argument(s) not defined correctly")
	}

	if len(clientCAPaths) == 0 {
		clientCAPaths = append(clientCAPaths, defaultClientCa)
	}

	if namespace = os.Getenv("NAMESPACE"); namespace == "" {
		namespace = "kube-system"
	}

	if !isValidPort(*healthCheckPort) {
		glog.Fatalf("Invalid health check port number. Choose between 1024 and 65535")
	} else if *healthCheckPort == *port {
		glog.Fatalf("Health check port should be different from port")
	} else {
		go func() {
			addr := fmt.Sprintf("%s:%d", *address, *healthCheckPort)
			mux := http.NewServeMux()

			mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			err := http.ListenAndServe(addr, mux)
			if err != nil {
				glog.Fatalf("error starting health check server: %v", err)
			}
		}()
	}

	glog.Infof("starting mutating admission controller for network resources injection")

	keyPair, err := webhook.NewTlsKeypairReloader(*cert, *key)
	if err != nil {
		glog.Fatalf("error load certificate: %s", err.Error())
	}

	clientCaPool, err := webhook.NewClientCertPool(&clientCAPaths, *insecure)
	if err != nil {
		glog.Fatalf("error loading client CA pool: '%s'", err.Error())
	}

	/* init API client */
	clientset := webhook.SetupInClusterClient()

	// initialize webhook with controlSwitches
	webhook.SetControlSwitches(controlSwitches)

	//initialize webhook with cache
	netAnnotationCache := netcache.Create()
	netAnnotationCache.Start()
	webhook.SetNetAttachDefCache(netAnnotationCache)

	userInjections := userdefinedinjections.CreateUserInjectionsStructure()
	webhook.SetUserInjectionStructure(userInjections)

	go func() {
		/* register handlers */
		var httpServer *http.Server

		http.HandleFunc("/mutate", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/mutate" {
				http.NotFound(w, r)
				return
			}
			if r.Method != http.MethodPost {
				http.Error(w, "Invalid HTTP verb requested", 405)
				return
			}
			webhook.MutateHandler(w, r)
		})

		/* start serving */
		httpServer = &http.Server{
			Addr:              fmt.Sprintf("%s:%d", *address, *port),
			ReadTimeout:       5 * time.Second,
			WriteTimeout:      10 * time.Second,
			MaxHeaderBytes:    1 << 20,
			ReadHeaderTimeout: 1 * time.Second,
			TLSConfig: &tls.Config{
				ClientAuth:               webhook.GetClientAuth(*insecure),
				MinVersion:               tls.VersionTLS12,
				CurvePreferences:         []tls.CurveID{tls.CurveP521, tls.CurveP384},
				ClientCAs:                clientCaPool.GetCertPool(),
				PreferServerCipherSuites: true,
				InsecureSkipVerify:       false,
				CipherSuites: []uint16{
					// tls 1.2
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					// tls 1.3 configuration not supported
				},
				GetCertificate: keyPair.GetCertificateFunc(),
			},
			// CVE-2023-39325 https://github.com/golang/go/issues/63417
			TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		}

		if *enableHTTP2 {
			httpServer.TLSNextProto = nil
		}

		err := httpServer.ListenAndServeTLS("", "")
		if err != nil {
			glog.Fatalf("error starting web server: %v", err)
		}
	}()

	/* watch the cert file and restart http sever if the file updated. */
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		glog.Fatalf("error starting fsnotify watcher: %v", err)
	}
	defer watcher.Close()

	certUpdated := false
	keyUpdated := false

	for {
		watcher.Add(*cert)
		watcher.Add(*key)

		select {
		case event, ok := <-watcher.Events:
			if !ok {
				continue
			}
			glog.V(2).Infof("watcher event: %v", event)
			mask := fsnotify.Create | fsnotify.Rename | fsnotify.Remove |
				fsnotify.Write | fsnotify.Chmod
			if (event.Op & mask) != 0 {
				glog.V(2).Infof("modified file: %v", event.Name)
				if event.Name == *cert {
					certUpdated = true
				}
				if event.Name == *key {
					keyUpdated = true
				}
				if keyUpdated && certUpdated {
					if err := keyPair.Reload(); err != nil {
						glog.Fatalf("Failed to reload certificate: %v", err)
					}
					certUpdated = false
					keyUpdated = false
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				continue
			}
			glog.Infof("watcher error: %v", err)
		case <-time.After(30 * time.Second):
			cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(
				context.Background(), controlSwitchesConfigMap, metav1.GetOptions{})
			// only in case of API errors report an error and do not restore default values
			if err != nil && !errors.IsNotFound(err) {
				glog.Warningf("Error getting control switches configmap %s", err.Error())
				continue
			}

			// to be called each time when map is present or not (in that case to restore default values)
			controlSwitches.ProcessControlSwitchesConfigMap(cm)
			userInjections.SetUserDefinedInjections(cm)
		}
	}

	// TODO: find a way to stop cache, should we run the above block in a go routine and make main module
	// to respond to terminate singal ?
}

func isValidPort(port int) bool {
	if port < 1024 || port > 65535 {
		return false
	}
	return true
}
