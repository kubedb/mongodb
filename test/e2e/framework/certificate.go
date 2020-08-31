/*
Copyright AppsCode Inc. and Contributors

Licensed under the PolyForm Noncommercial License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/PolyForm-Noncommercial-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"context"
	"path/filepath"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

	"github.com/appscode/go/ioutil"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	certPath = "/tmp/mongodb/"
)

// GetSSLCertificate gets ssl certificate of mongodb and creates a client certificate in certPath
func (f *Framework) GetSSLCertificate(meta metav1.ObjectMeta) error {
	mg, err := f.GetMongoDB(meta)
	if err != nil {
		return err
	}

	certSecret, err := f.kubeClient.CoreV1().Secrets(mg.Namespace).Get(context.TODO(), mg.MustCertSecretName(api.MongoDBClientCert, ""), metav1.GetOptions{})
	if err != nil {
		return err
	}

	caCertBytes := certSecret.Data[string(api.TLSCACertFileName)]
	certBytes := append(certSecret.Data[string(tlsCertFileKey)], certSecret.Data[string(tlsKeyFileKey)]...)

	if !ioutil.WriteString(filepath.Join(certPath, string(api.TLSCACertFileName)), string(caCertBytes)) {
		return errors.New("failed to write client certificate")
	}

	if !ioutil.WriteString(filepath.Join(certPath, string(api.MongoPemFileName)), string(certBytes)) {
		return errors.New("failed to write client certificate")
	}

	return nil
}
