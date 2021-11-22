// Copyright (c) 2018 Intel Corporation
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

package installer

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/cloudflare/cfssl/csr"
	"github.com/cloudflare/cfssl/helpers"
	"github.com/cloudflare/cfssl/initca"
	cfsigner "github.com/cloudflare/cfssl/signer"
	"github.com/cloudflare/cfssl/signer/local"
	"github.com/golang/glog"
	"github.com/pkg/errors"

	arv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	clientset kubernetes.Interface
	namespace string
	prefix    string
)

const keyBitLength = 3072
const CAExpiration = "630720000s"

func generateCSR() ([]byte, []byte, error) {
	glog.Infof("generating Certificate Signing Request")
	serviceName := strings.Join([]string{prefix, "service"}, "-")
	certRequest := csr.New()
	certRequest.KeyRequest = &csr.KeyRequest{A: "rsa", S: keyBitLength}
	certRequest.CN = strings.Join([]string{serviceName, namespace, "svc"}, ".")
	certRequest.Hosts = []string{
		serviceName,
		strings.Join([]string{serviceName, namespace}, "."),
		strings.Join([]string{serviceName, namespace, "svc"}, "."),
	}
	return csr.ParseRequest(certRequest)
}

func generateCACertificate() (*local.Signer, []byte, error) {
	certRequest := csr.New()
	certRequest.KeyRequest = &csr.KeyRequest{A: "rsa", S: keyBitLength}
	certRequest.CN = "Kubernetes NRI"
	certRequest.CA = &csr.CAConfig{Expiry: CAExpiration}
	cert, _, key, err := initca.New(certRequest)
	if err != nil {
		return nil, nil, fmt.Errorf("creating CA certificate failed: %v", err)
	}
	parsedKey, err := helpers.ParsePrivateKeyPEM(key)
	if err != nil {
		return nil, nil, fmt.Errorf("parsing private key pem failed: %v", err)
	}
	parsedCert, err := helpers.ParseCertificatePEM(cert)
	if err != nil {
		return nil, nil, fmt.Errorf("parse certificate failed: %v", err)
	}
	signer, err := local.NewSigner(parsedKey, parsedCert, cfsigner.DefaultSigAlgo(parsedKey), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create signer: %v", err)
	}
	return signer, cert, nil
}

func writeToFile(certificate, key []byte, certFilename, keyFilename string) error {
	if err := ioutil.WriteFile("/etc/tls/"+certFilename, certificate, 0400); err != nil {
		return err
	}
	if err := ioutil.WriteFile("/etc/tls/"+keyFilename, key, 0400); err != nil {
		return err
	}
	return nil
}

func createMutatingWebhookConfiguration(certificate []byte, failurePolicyStr string) error {
	configName := strings.Join([]string{prefix, "mutating-config"}, "-")
	serviceName := strings.Join([]string{prefix, "service"}, "-")
	removeMutatingWebhookIfExists(configName)
	var failurePolicy arv1.FailurePolicyType
	if strings.EqualFold(strings.TrimSpace(failurePolicyStr), "Ignore") {
		failurePolicy = arv1.Ignore
	} else if strings.EqualFold(strings.TrimSpace(failurePolicyStr), "Fail") {
		failurePolicy = arv1.Fail
	} else {
		return errors.New("unknown failure policy type")
	}
	sideEffects := arv1.SideEffectClassNone
	path := "/mutate"
	namespaces := []string{"kube-system"}
	if namespace != "kube-system" {
		namespaces = append(namespaces, namespace)
	}
	namespaceSelector := metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{
				Key:      "kubernetes.io/metadata.name",
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   namespaces,
			},
		},
	}
	configuration := &arv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: configName,
			Labels: map[string]string{
				"app": prefix,
			},
		},
		Webhooks: []arv1.MutatingWebhook{
			arv1.MutatingWebhook{
				Name: configName + ".k8s.cni.cncf.io",
				ClientConfig: arv1.WebhookClientConfig{
					CABundle: certificate,
					Service: &arv1.ServiceReference{
						Namespace: namespace,
						Name:      serviceName,
						Path:      &path,
					},
				},
				FailurePolicy:           &failurePolicy,
				AdmissionReviewVersions: []string{"v1"},
				SideEffects:             &sideEffects,
				NamespaceSelector:       &namespaceSelector,
				Rules: []arv1.RuleWithOperations{
					arv1.RuleWithOperations{
						Operations: []arv1.OperationType{arv1.Create},
						Rule: arv1.Rule{
							APIGroups:   []string{""},
							APIVersions: []string{"v1"},
							Resources:   []string{"pods"},
						},
					},
				},
			},
		},
	}
	_, err := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(context.TODO(), configuration, metav1.CreateOptions{})
	return err
}

func createService() error {
	serviceName := strings.Join([]string{prefix, "service"}, "-")
	removeServiceIfExists(serviceName)
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
			Labels: map[string]string{
				"app": prefix,
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Port:       443,
					TargetPort: intstr.FromInt(8443),
				},
			},
			Selector: map[string]string{
				"app": prefix,
			},
		},
	}
	_, err := clientset.CoreV1().Services(namespace).Create(context.TODO(), service, metav1.CreateOptions{})
	return err
}

func removeServiceIfExists(serviceName string) {
	service, err := clientset.CoreV1().Services(namespace).Get(context.TODO(), serviceName, metav1.GetOptions{})
	if service != nil && err == nil {
		glog.Infof("service %s already exists, removing it first", serviceName)
		err := clientset.CoreV1().Services(namespace).Delete(context.TODO(), serviceName, metav1.DeleteOptions{})
		if err != nil {
			glog.Errorf("error trying to remove service: %s", err)
		}
		glog.Infof("service %s removed", serviceName)
	}
}

func removeMutatingWebhookIfExists(configName string) {
	config, err := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(context.TODO(), configName, metav1.GetOptions{})
	if config != nil && err == nil {
		glog.Infof("mutating webhook %s already exists, removing it first", configName)
		err := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(context.TODO(), configName, metav1.DeleteOptions{})
		if err != nil {
			glog.Errorf("error trying to remove mutating webhook configuration: %s", err)
		}
		glog.Infof("mutating webhook configuration %s removed", configName)
	}
}

func removeSecretIfExists(secretName string) {
	secret, err := clientset.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if secret != nil && err == nil {
		glog.Infof("secret %s already exists, removing it first", secretName)
		err := clientset.CoreV1().Secrets(namespace).Delete(context.TODO(), secretName, metav1.DeleteOptions{})
		if err != nil {
			glog.Errorf("error trying to remove secret: %s", err)
		}
		glog.Infof("secret %s removed", secretName)
	}
}

// Install creates resources required by mutating admission webhook
func Install(k8sNamespace, namePrefix, failurePolicy string) {
	/* setup Kubernetes API client */
	config, err := rest.InClusterConfig()
	if err != nil {
		glog.Fatalf("error loading Kubernetes in-cluster configuration: %s", err)
	}
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		glog.Fatalf("error setting up Kubernetes client: %s", err)
	}

	namespace = k8sNamespace
	prefix = namePrefix

	signer, caCertificate, err := generateCACertificate()
	if err != nil {
		glog.Fatalf("Error generating CA certificate and signer: %s", err)
	}

	/* generate CSR and private key */
	csr, key, err := generateCSR()
	if err != nil {
		glog.Fatalf("error generating CSR and private key: %s", err)
	}
	glog.Infof("raw CSR and private key successfully created")

	certificate, err := signer.Sign(cfsigner.SignRequest{
		Request: string(csr),
	})
	if err != nil {
		glog.Fatalf("error getting signed certificate: %s", err)
	}
	glog.Infof("signed certificate successfully obtained")

	err = writeToFile(certificate, key, "tls.crt", "tls.key")
	if err != nil {
		glog.Fatalf("error writing certificate and key to files: %s", err)
	}
	glog.Infof("certificate and key written to files")

	/* create webhook configurations */
	err = createMutatingWebhookConfiguration(caCertificate, failurePolicy)
	if err != nil {
		glog.Fatalf("error creating mutating webhook configuration: %s", err)
	}
	glog.Infof("mutating webhook configuration successfully created")

	/* create service */
	err = createService()
	if err != nil {
		glog.Fatalf("error creating service: %s", err)
	}
	glog.Infof("service successfully created")

	glog.Infof("all resources created successfully")
}
