package controller

import (
	"fmt"

	"github.com/appscode/go/log"
	"github.com/appscode/kutil"
	meta_util "github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	"github.com/kubedb/apimachinery/pkg/eventer"
	"github.com/kubedb/apimachinery/pkg/storage"
	"github.com/kubedb/mongodb/pkg/validator"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Controller) create(mongodb *api.MongoDB) error {
	if err := validator.ValidateMongoDB(c.Client, c.ExtClient, mongodb); err != nil {
		c.recorder.Event(mongodb.ObjectReference(), core.EventTypeWarning, eventer.EventReasonInvalid, err.Error())
		log.Errorln(err)
		return nil
	}

	// Delete Matching DormantDatabase if exists any
	if err := c.killMatchingDormantDatabase(mongodb); err != nil {
		c.recorder.Eventf(mongodb.ObjectReference(), core.EventTypeWarning, eventer.EventReasonFailedToCreate,
			`Failed to delete dormant Database : "%v". Reason: %v`, mongodb.Name, err,
		)
		return err
	}

	if mongodb.Status.CreationTime == nil {
		mg, _, err := util.PatchMongoDB(c.ExtClient, mongodb, func(in *api.MongoDB) *api.MongoDB {
			t := metav1.Now()
			in.Status.CreationTime = &t
			in.Status.Phase = api.DatabasePhaseCreating
			return in
		})
		if err != nil {
			c.recorder.Eventf(mongodb.ObjectReference(), core.EventTypeWarning, eventer.EventReasonFailedToUpdate, err.Error())
			return err
		}
		mongodb.Status = mg.Status
	}

	// create Governing Service
	governingService := c.opt.GoverningService
	if err := c.CreateGoverningService(governingService, mongodb.Namespace); err != nil {
		c.recorder.Eventf(mongodb.ObjectReference(), core.EventTypeWarning, eventer.EventReasonFailedToCreate,
			`Failed to create Service: "%v". Reason: %v`, governingService, err,
		)
		return err
	}

	// ensure database Service
	vt1, err := c.ensureService(mongodb)
	if err != nil {
		return err
	}

	if err := c.ensureDatabaseSecret(mongodb); err != nil {
		return err
	}

	// ensure database StatefulSet
	vt2, err := c.ensureStatefulSet(mongodb)
	if err != nil {
		return err
	}

	if vt1 == kutil.VerbCreated && vt2 == kutil.VerbCreated {
		c.recorder.Event(mongodb.ObjectReference(), core.EventTypeNormal, eventer.EventReasonSuccessful,
			"Successfully created MongoDB",
		)
	} else if vt1 == kutil.VerbPatched || vt2 == kutil.VerbPatched {
		c.recorder.Event(mongodb.ObjectReference(), core.EventTypeNormal, eventer.EventReasonSuccessful,
			"Successfully patched MongoDB",
		)
	}

	if _, err := meta_util.GetString(mongodb.Annotations, api.AnnotationInitialized); err == kutil.ErrNotFound &&
		mongodb.Spec.Init != nil && mongodb.Spec.Init.SnapshotSource != nil {

		snapshotSource := mongodb.Spec.Init.SnapshotSource

		if mongodb.Status.Phase == api.DatabasePhaseInitializing {
			return nil
		}
		jobName := fmt.Sprintf("%s-%s", api.DatabaseNamePrefix, snapshotSource.Name)
		if _, err := c.Client.BatchV1().Jobs(snapshotSource.Namespace).Get(jobName, metav1.GetOptions{}); err != nil {
			if kerr.IsAlreadyExists(err) {
				return nil
			} else if !kerr.IsNotFound(err) {
				return err
			}
		}
		if err := c.initialize(mongodb); err != nil {
			return fmt.Errorf("failed to complete initialization. Reason: %v", err)
		}
		return nil
	}

	ms, _, err := util.PatchMongoDB(c.ExtClient, mongodb, func(in *api.MongoDB) *api.MongoDB {
		in.Status.Phase = api.DatabasePhaseRunning
		return in
	})
	if err != nil {
		c.recorder.Eventf(mongodb, core.EventTypeWarning, eventer.EventReasonFailedToUpdate, err.Error())
		return err
	}
	mongodb.Status = ms.Status

	// Ensure Schedule backup
	c.ensureBackupScheduler(mongodb)

	if err := c.manageMonitor(mongodb); err != nil {
		c.recorder.Eventf(mongodb.ObjectReference(), core.EventTypeWarning, eventer.EventReasonFailedToCreate,
			"Failed to manage monitoring system. Reason: %v", err,
		)
		log.Errorln(err)
		return nil
	}

	return nil
}

func (c *Controller) ensureBackupScheduler(mongodb *api.MongoDB) {
	// Setup Schedule backup
	if mongodb.Spec.BackupSchedule != nil {
		err := c.cronController.ScheduleBackup(mongodb, mongodb.ObjectMeta, mongodb.Spec.BackupSchedule)
		if err != nil {
			c.recorder.Eventf(mongodb.ObjectReference(), core.EventTypeWarning, eventer.EventReasonFailedToSchedule,
				"Failed to schedule snapshot. Reason: %v", err,
			)
			log.Errorln(err)
		}
	} else {
		c.cronController.StopBackupScheduling(mongodb.ObjectMeta)
	}
}

func (c *Controller) initialize(mongodb *api.MongoDB) error {
	mg, _, err := util.PatchMongoDB(c.ExtClient, mongodb, func(in *api.MongoDB) *api.MongoDB {
		in.Status.Phase = api.DatabasePhaseInitializing
		return in
	})
	if err != nil {
		c.recorder.Eventf(mongodb.ObjectReference(), core.EventTypeWarning, eventer.EventReasonFailedToUpdate, err.Error())
		return err
	}
	mongodb.Status = mg.Status

	snapshotSource := mongodb.Spec.Init.SnapshotSource
	// Event for notification that kubernetes objects are creating
	c.recorder.Eventf(mongodb.ObjectReference(), core.EventTypeNormal, eventer.EventReasonInitializing,
		`Initializing from Snapshot: "%v"`, snapshotSource.Name,
	)

	namespace := snapshotSource.Namespace
	if namespace == "" {
		namespace = mongodb.Namespace
	}
	snapshot, err := c.ExtClient.Snapshots(namespace).Get(snapshotSource.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	secret, err := storage.NewOSMSecret(c.Client, snapshot)
	if err != nil {
		return err
	}
	secret, err = c.Client.CoreV1().Secrets(secret.Namespace).Create(secret)
	if err != nil && !kerr.IsAlreadyExists(err) {
		return err
	}

	job, err := c.createRestoreJob(mongodb, snapshot)
	if err != nil {
		return err
	}

	if err := c.SetJobOwnerReference(snapshot, job); err != nil {
		return err
	}

	return nil
}

func (c *Controller) pause(mongodb *api.MongoDB) error {
	if _, err := c.createDormantDatabase(mongodb); err != nil {
		if kerr.IsAlreadyExists(err) {
			// if already exists, check if it is database of another Kind and return error in that case.
			// If the Kind is same, we can safely assume that the DormantDB was not deleted in before,
			// Probably because, User is more faster (create-delete-create-again-delete...) than operator!
			// So reuse that DormantDB!
			ddb, err := c.ExtClient.DormantDatabases(mongodb.Namespace).Get(mongodb.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}

			if val, _ := meta_util.GetStringValue(ddb.Labels, api.LabelDatabaseKind); val != api.ResourceKindMongoDB {
				return fmt.Errorf(`DormantDatabase "%v" of kind %v already exists`, mongodb.Name, val)
			}
		} else {
			return fmt.Errorf(`Failed to create DormantDatabase: "%v". Reason: %v`, mongodb.Name, err)
		}
	}
	c.recorder.Eventf(mongodb.ObjectReference(), core.EventTypeNormal, eventer.EventReasonSuccessfulCreate,
		`Successfully created DormantDatabase: "%v"`, mongodb.Name,
	)

	c.cronController.StopBackupScheduling(mongodb.ObjectMeta)

	if mongodb.Spec.Monitor != nil {
		if _, err := c.deleteMonitor(mongodb); err != nil {
			c.recorder.Eventf(mongodb.ObjectReference(), core.EventTypeWarning, eventer.EventReasonFailedToDelete,
				"Failed to delete monitoring system. Reason: %v", err,
			)
			log.Errorln(err)
			return nil
		}
	}
	return nil
}
