package framework

import (
	"path/filepath"

	"github.com/appscode/go/ioutil"
	"github.com/pkg/errors"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
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
