package controller

import (
	"errors"

	"github.com/appscode/go/log"
	tapi "github.com/k8sdb/apimachinery/apis/kubedb/v1alpha1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (c *Controller) Exists(om *metav1.ObjectMeta) (bool, error) {
	if _, err := c.ExtClient.MongoDBs(om.Namespace).Get(om.Name, metav1.GetOptions{}); err != nil {
		if !kerr.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}

	return true, nil
}

func (c *Controller) PauseDatabase(dormantDb *tapi.DormantDatabase) error {
	// Delete Service
	if err := c.DeleteService(dormantDb.Name, dormantDb.Namespace); err != nil {
		log.Errorln(err)
		return err
	}

	if err := c.DeleteStatefulSet(dormantDb.OffshootName(), dormantDb.Namespace); err != nil {
		log.Errorln(err)
		return err
	}

	mongodb := &tapi.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dormantDb.OffshootName(),
			Namespace: dormantDb.Namespace,
		},
	}
	if err := c.deleteRBACStuff(mongodb); err != nil {
		log.Errorln(err)
		return err
	}
	return nil
}

func (c *Controller) WipeOutDatabase(dormantDb *tapi.DormantDatabase) error {
	labelMap := map[string]string{
		tapi.LabelDatabaseName: dormantDb.Name,
		tapi.LabelDatabaseKind: tapi.ResourceKindMongoDB,
	}

	labelSelector := labels.SelectorFromSet(labelMap)

	if err := c.DeleteSnapshots(dormantDb.Namespace, labelSelector); err != nil {
		log.Errorln(err)
		return err
	}

	if err := c.DeletePersistentVolumeClaims(dormantDb.Namespace, labelSelector); err != nil {
		log.Errorln(err)
		return err
	}

	// ---> Start
	//TODO: Use following to delete secret, if appropriate
	// Otherwise, remove it
	/*
		if dormantDb.Spec.Origin.Spec.MongoDB.DatabaseSecret != nil {
			if err := c.deleteSecret(dormantDb); err != nil {
				return err
			}

		}
	*/
	// ---> End

	return nil
}

// ---> Start
//TODO: Use this method to delete secret, if supported
// Otherwise, remove it
func (c *Controller) deleteSecret(dormantDb *tapi.DormantDatabase) error {

	var secretFound bool = false

	dormantDatabaseSecret := dormantDb.Spec.Origin.Spec.MongoDB.DatabaseSecret

	mongodbList, err := c.ExtClient.MongoDBs(dormantDb.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, mongodb := range mongodbList.Items {
		databaseSecret := mongodb.Spec.DatabaseSecret
		if databaseSecret != nil {
			if databaseSecret.SecretName == dormantDatabaseSecret.SecretName {
				secretFound = true
				break
			}
		}
	}

	if !secretFound {
		labelMap := map[string]string{
			tapi.LabelDatabaseKind: tapi.ResourceKindMongoDB,
		}
		dormantDatabaseList, err := c.ExtClient.DormantDatabases(dormantDb.Namespace).List(
			metav1.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelMap).String(),
			},
		)
		if err != nil {
			return err
		}

		for _, ddb := range dormantDatabaseList.Items {
			if ddb.Name == dormantDb.Name {
				continue
			}

			databaseSecret := ddb.Spec.Origin.Spec.MongoDB.DatabaseSecret
			if databaseSecret != nil {
				if databaseSecret.SecretName == dormantDatabaseSecret.SecretName {
					secretFound = true
					break
				}
			}

		}
	}

	if !secretFound {
		if err := c.DeleteSecret(dormantDatabaseSecret.SecretName, dormantDb.Namespace); err != nil {
			return err
		}
	}

	return nil
}

// ---> End

func (c *Controller) ResumeDatabase(dormantDb *tapi.DormantDatabase) error {
	origin := dormantDb.Spec.Origin
	objectMeta := origin.ObjectMeta

	if origin.Spec.MongoDB.Init != nil {
		return errors.New("do not support InitSpec in spec.origin.mongodb")
	}

	mongodb := &tapi.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:        objectMeta.Name,
			Namespace:   objectMeta.Namespace,
			Labels:      objectMeta.Labels,
			Annotations: objectMeta.Annotations,
		},
		Spec: *origin.Spec.MongoDB,
	}

	if mongodb.Annotations == nil {
		mongodb.Annotations = make(map[string]string)
	}

	for key, val := range dormantDb.Annotations {
		mongodb.Annotations[key] = val
	}

	_, err := c.ExtClient.MongoDBs(mongodb.Namespace).Create(mongodb)
	return err
}
