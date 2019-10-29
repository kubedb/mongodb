/*
Copyright The KubeDB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package framework

import (
	"path/filepath"

	"kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

	"github.com/appscode/go/ioutil"
	"github.com/pkg/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	certPath = "/tmp/mongodb/"
)

// GetSSLCertificate gets ssl certificate of mongodb and creates a client certificate in certPath
func (f *Framework) GetSSLCertificate(meta v12.ObjectMeta) error {
	mg, err := f.GetMongoDB(meta)
	if err != nil {
		return err
	}

	certSecret, err := f.kubeClient.CoreV1().Secrets(mg.Namespace).Get(mg.Spec.CertificateSecret.SecretName, v12.GetOptions{})
	if err != nil {
		return err
	}

	caCertBytes := certSecret.Data[string(v1alpha1.MongoTLSCertFileName)]
	certBytes := certSecret.Data[string(v1alpha1.MongoClientPemFileName)]

	if !ioutil.WriteString(filepath.Join(certPath, string(v1alpha1.MongoTLSCertFileName)), string(caCertBytes)) {
		return errors.New("failed to write client certificate")
	}

	if !ioutil.WriteString(filepath.Join(certPath, string(v1alpha1.MongoClientPemFileName)), string(certBytes)) {
		return errors.New("failed to write client certificate")
	}

	return nil
}
