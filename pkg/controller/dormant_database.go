package controller

import (
	core_util "github.com/appscode/kutil/core/v1"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) ExDatabaseStatus(dormantDb *api.DormantDatabase) error {
	mongodb := &api.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dormantDb.OffshootName(),
			Namespace: dormantDb.Namespace,
		},
	}

	statefulSet, err := c.Client.AppsV1().StatefulSets(mongodb.Namespace).Get(mongodb.Name, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return nil
		} else {
			return err
		}
	}

	if err = core_util.WaitUntilPodDeletedBySelector(c.Client, statefulSet.Namespace, statefulSet.Spec.Selector); err != nil {
		return err
	}

	//todo: make sure service is deleted as welll
	//_, err = c.Client.CoreV1().Services(mongodb.Namespace).Get(mongodb.Name, metav1.GetOptions{})
	//if err != nil {
	//	if kerr.IsNotFound(err) {
	//		return nil
	//	} else {
	//		return err
	//	}
	//}

	return nil
}
