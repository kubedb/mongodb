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
	"fmt"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/log"
	cm_api "github.com/jetstack/cert-manager/pkg/apis/certmanager/v1alpha2"
	v1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	IssuerName     = "e2e-self-signed-issuer"
	tlsCertFileKey = "tls.crt"
	tlsKeyFileKey  = "tls.key"
)

func (f *Framework) IssuerForMongoDB(mgoMeta, caSecretMeta metav1.ObjectMeta) *cm_api.Issuer {
	thisIssuerName := rand.WithUniqSuffix(IssuerName)
	labelMap := map[string]string{
		api.LabelDatabaseName: mgoMeta.Name,
		api.LabelDatabaseKind: api.ResourceKindMongoDB,
	}
	return &cm_api.Issuer{
		TypeMeta: metav1.TypeMeta{
			Kind: cm_api.IssuerKind,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      thisIssuerName,
			Namespace: mgoMeta.Namespace,
			Labels:    labelMap,
		},
		Spec: cm_api.IssuerSpec{
			IssuerConfig: cm_api.IssuerConfig{
				CA: &cm_api.CAIssuer{
					SecretName: caSecretMeta.Name,
				},
			},
		},
	}
}

func (f *Framework) CreateIssuer(obj *cm_api.Issuer) error {
	_, err := f.certManagerClient.CertmanagerV1alpha2().Issuers(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) UpdateIssuer(meta metav1.ObjectMeta, transformer func(cm_api.Issuer) cm_api.Issuer) error {
	attempt := 0
	for ; attempt < maxAttempts; attempt = attempt + 1 {
		cur, err := f.certManagerClient.CertmanagerV1alpha2().Issuers(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(err) {
			return nil
		} else if err == nil {
			modified := transformer(*cur)
			_, err = f.certManagerClient.CertmanagerV1alpha2().Issuers(cur.Namespace).Update(&modified)
			if err == nil {
				return nil
			}
		}
		log.Errorf("Attempt %d failed to update Issuer %s@%s due to %s.", attempt, cur.Name, cur.Namespace, err)
		time.Sleep(updateRetryInterval)
	}
	return fmt.Errorf("failed to update Issuer %s@%s after %d attempts", meta.Name, meta.Namespace, attempt)
}

func (f *Framework) DeleteIssuer(meta metav1.ObjectMeta) error {
	return f.certManagerClient.CertmanagerV1alpha2().Issuers(meta.Namespace).Delete(meta.Name, deleteInForeground())
}

func (f *Framework) SelfSignedCASecret(meta metav1.ObjectMeta) *v1.Secret {
	labelMap := map[string]string{
		api.LabelDatabaseName: meta.Name,
		api.LabelDatabaseKind: api.ResourceKindMongoDB,
	}
	return &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix(meta.Name) + "-ca",
			Namespace: meta.Namespace,
			Labels:    labelMap,
		},
		Data: map[string][]byte{
			tlsCertFileKey: f.CertStore.CACertBytes(),
			tlsKeyFileKey:  f.CertStore.CAKeyBytes(),
		},
	}
}
