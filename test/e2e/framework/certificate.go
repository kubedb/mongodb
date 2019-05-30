package framework

import (
	"crypto/x509"
	"encoding/pem"
	"path/filepath"

	"github.com/appscode/go/ioutil"
	"github.com/kubedb/mongodb/pkg/controller"
	"github.com/pkg/errors"
	"gomodules.xyz/cert"
	v12 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	certPath   = "/tmp/mongodb/"
	clientCert = "client.pem"
)

// GetSSLCertificate gets ssl certificate of mongodb and creates a client certificate in certPath
func (f *Framework) GetSSLCertificate(meta v12.ObjectMeta) error {
	cfg := cert.Config{
		CommonName:   meta.Name,
		Organization: []string{"MongoDB Operator"},
		Usages: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
			x509.ExtKeyUsageClientAuth,
		},
	}

	mg, err := f.GetMongoDB(meta)
	if err != nil {
		return err
	}

	certSecret, err := f.kubeClient.CoreV1().Secrets(mg.Namespace).Get(mg.Spec.CertificateSecret.SecretName, v12.GetOptions{})
	if err != nil {
		return err
	}

	caCertBytes := certSecret.Data[string(controller.TLSCert)]

	caKeyBlocks, _ := pem.Decode(certSecret.Data[string(controller.TLSKey)])
	caCertBlocks, _ := pem.Decode(certSecret.Data[string(controller.TLSCert)])

	if !ioutil.WriteString(filepath.Join(certPath, string(controller.TLSCert)), string(caCertBytes)) {
		return errors.New("failed to write client certificate")
	}

	caKey, err := x509.ParsePKCS1PrivateKey(caKeyBlocks.Bytes)
	if err != nil {
		return err
	}

	caCert, err := x509.ParseCertificate(caCertBlocks.Bytes)
	if err != nil {
		return err
	}

	clientPrivateKey, err := cert.NewPrivateKey()
	if err != nil {
		return errors.New("failed to generate key for client certificate")
	}

	clientCertificate, err := cert.NewSignedCert(cfg, clientPrivateKey, caCert, caKey)
	if err != nil {
		return errors.New("failed to sign client certificate")
	}

	clientKeyByte := cert.EncodePrivateKeyPEM(clientPrivateKey)
	clientCertByte := cert.EncodeCertPEM(clientCertificate)
	certBytes := append(clientCertByte, clientKeyByte...)

	if !ioutil.WriteString(filepath.Join(certPath, string(clientCert)), string(certBytes)) {
		return errors.New("failed to write client certificate")
	}

	return nil
}
