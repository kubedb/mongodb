package controller

import (
	"github.com/appscode/go/crypto/rand"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/pkg/eventer"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	mongodbUser = "root"

	KeyMongoDBUser     = "user"
	KeyMongoDBPassword = "password"

	ExporterSecretPath = "/var/run/secrets/kubedb.com/"
)

func (c *Controller) ensureDatabaseSecret(mongodb *api.MongoDB) error {

	authSecretName := mongodb.Name + "-auth"

	sc, err := c.checkSecret(authSecretName, mongodb)
	if err != nil {
		return err
	}

	if sc == nil {
		if err := c.createDatabaseSecret(mongodb); err != nil {
			c.recorder.Eventf(
				mongodb.ObjectReference(),
				core.EventTypeWarning,
				eventer.EventReasonFailedToCreate,
				`Failed to create Database Secret. Reason: %v`,
				err.Error(),
			)
			return err
		}
	}
	return nil
}

func (c *Controller) createDatabaseSecret(mongodb *api.MongoDB) error {
	authSecretName := mongodb.Name + "-auth"

	randPassword := ""
	for randPassword = rand.GeneratePassword(); randPassword[0] == '-'; {
	}

	secret := &core.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: authSecretName,
			Labels: map[string]string{
				api.LabelDatabaseKind: api.ResourceKindMongoDB,
				api.LabelDatabaseName: mongodb.Name,
			},
		},
		Type: core.SecretTypeOpaque,
		StringData: map[string]string{
			KeyMongoDBUser:     mongodbUser,
			KeyMongoDBPassword: randPassword,
		},
	}
	if _, err := c.Client.CoreV1().Secrets(mongodb.Namespace).Create(secret); err != nil {
		return err
	}
	return nil
}

func (c *Controller) checkSecret(secretName string, mongodb *api.MongoDB) (*core.Secret, error) {
	secret, err := c.Client.CoreV1().Secrets(mongodb.Namespace).Get(secretName, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}
	//if secret.Labels[api.LabelDatabaseKind] != api.ResourceKindMongoDB ||
	//	secret.Labels[api.LabelDatabaseName] != mongodb.Name {
	//	return nil, fmt.Errorf(`intended secret "%v" already exists`, secretName)
	//}

	return secret, nil
}
