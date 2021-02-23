// Copyright (c) 2019 Multus Authors
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

package webhook

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/golang/glog"
)

type tlsKeypairReloader struct {
	certMutex sync.RWMutex
	cert      *tls.Certificate
	certPath  string
	keyPath   string
}

type clientCertPool struct {
	certPool  *x509.CertPool
	certPaths *ClientCAFlags
	insecure  bool
}

type ClientCAFlags []string

func (i *ClientCAFlags) String() string {
	return ""
}

func (i *ClientCAFlags) Set(path string) error {
	*i = append(*i, path)
	return nil
}

func (keyPair *tlsKeypairReloader) Reload() error {
	newCert, err := tls.LoadX509KeyPair(keyPair.certPath, keyPair.keyPath)
	if err != nil {
		return err
	}
	glog.V(2).Infof("cetificate reloaded")
	keyPair.certMutex.Lock()
	defer keyPair.certMutex.Unlock()
	keyPair.cert = &newCert
	return nil
}

func (keyPair *tlsKeypairReloader) GetCertificateFunc() func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	return func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
		keyPair.certMutex.RLock()
		defer keyPair.certMutex.RUnlock()
		return keyPair.cert, nil
	}
}

// reload tlsKeypairReloader struct
func NewTLSKeypairReloader(certPath, keyPath string) (*tlsKeypairReloader, error) {
	result := &tlsKeypairReloader{
		certPath: certPath,
		keyPath:  keyPath,
	}
	cert, err := tls.LoadX509KeyPair(certPath, keyPath)
	if err != nil {
		return nil, err
	}
	result.cert = &cert

	return result, nil
}

//NewClientCertPool will load a single client CA
func NewClientCertPool(clientCaPaths *ClientCAFlags, insecure bool) (*clientCertPool, error) {
	pool := &clientCertPool{
		certPaths: clientCaPaths,
		insecure:  insecure,
	}
	if !pool.insecure {
		if err := pool.Load(); err != nil {
			return nil, err
		}
	}
	return pool, nil
}

//Load a certificate into the client CA pool
func (pool *clientCertPool) Load() error {
	if pool.insecure {
		glog.Infof("can not load client CA pool. Remove --insecure flag to enable.")
		return nil
	}

	if len(*pool.certPaths) == 0 {
		return fmt.Errorf("no client CA file path(s) found")
	}

	pool.certPool = x509.NewCertPool()
	for _, path := range *pool.certPaths {
		caCertPem, err := ioutil.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to load client CA file from path '%s'", path)
		}
		if ok := pool.certPool.AppendCertsFromPEM(caCertPem); !ok {
			return fmt.Errorf("failed to parse client CA file from path '%s'", path)
		}
		glog.Infof("added client CA to cert pool from path '%s'", path)
	}
	glog.Infof("added '%d' client CA(s) to cert pool", len(*pool.certPaths))
	return nil
}

//GetCertPool returns a client CA pool
func (pool *clientCertPool) GetCertPool() *x509.CertPool {
	if pool.insecure {
		return nil
	}
	return pool.certPool
}

//GetClientAuth determines the policy the http server will follow for TLS Client Authentication
func GetClientAuth(insecure bool) tls.ClientAuthType {
	if insecure {
		return tls.NoClientCert
	}
	return tls.RequireAndVerifyClientCert
}
