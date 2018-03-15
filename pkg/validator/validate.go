package validator

import (
	"fmt"

	"github.com/appscode/go/types"
	core_util "github.com/appscode/kutil/core/v1"
	meta_util "github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	cs "github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1"
	amv "github.com/kubedb/apimachinery/pkg/validator"
	coreV1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
)

var (
	mongodbVersions = sets.NewString("3.4", "3.6")
)

func OnCreateValidate(client kubernetes.Interface, extClient cs.KubedbV1alpha1Interface, mongodb *api.MongoDB) error {
	if mongodb.Spec.Version == "" {
		return fmt.Errorf(`object 'Version' is missing in '%v'`, mongodb.Spec)
	}

	// Check MongoDB version validation
	if !mongodbVersions.Has(string(mongodb.Spec.Version)) {
		return fmt.Errorf(`KubeDB doesn't support MongoDB version: %s`, string(mongodb.Spec.Version))
	}

	if mongodb.Spec.Replicas != nil {
		replicas := types.Int32(mongodb.Spec.Replicas)
		if replicas != 1 {
			return fmt.Errorf(`spec.replicas "%d" invalid. Value must be one`, replicas)
		}
	}

	if mongodb.Spec.Storage != nil {
		var err error
		if err = amv.ValidateStorage(client, mongodb.Spec.Storage); err != nil {
			return err
		}
	}

	backupScheduleSpec := mongodb.Spec.BackupSchedule
	if backupScheduleSpec != nil {
		if err := amv.ValidateBackupSchedule(client, backupScheduleSpec, mongodb.Namespace); err != nil {
			return err
		}
	}

	monitorSpec := mongodb.Spec.Monitor
	if monitorSpec != nil {
		if err := amv.ValidateMonitorSpec(monitorSpec); err != nil {
			return err
		}
	}
	return nil
}

// OnCreateLeftovers does the additional tasks that is related to editing or deleting other Kinds of Objects
func OnCreateLeftOvers(extClient cs.KubedbV1alpha1Interface, mongodb *api.MongoDB) error {
	// Delete matching Dormant Database
	if err := killMatchingDormantDatabase(extClient, mongodb); err != nil {
		return err
	}

	return nil
}

// OnCreate does the additional tasks that is related to editing or deleting other Kinds of Objects
//
// Major Tasks:
// - Create Dormant Database with Finalizer
// Let kubernetes Garbage Collect of StatefulSets, Service
func OnDeleteLeftOvers(client kubernetes.Interface, extClient cs.KubedbV1alpha1Interface, mongodb api.MongoDB) (runtime.Object, error) {
	ddb := getDormantDatabase(&mongodb)
	if _, err := extClient.DormantDatabases(ddb.Namespace).Create(ddb); err != nil {
		return nil, err
	}

	return &mongodb, nil
}

// SterilizeSecrets cleans secret that is created for this Ex-MongoDB database by KubeDB-Operator and
// not used by any other MongoDB or DormantDatabases objects.
func SterilizeSecrets(client kubernetes.Interface, extClient cs.KubedbV1alpha1Interface, ddb *api.DormantDatabase) error {
	secretFound := false

	secretVolume := ddb.Spec.Origin.Spec.MongoDB.DatabaseSecret
	if secretVolume == nil {
		return nil
	}

	secret, err := client.CoreV1().Secrets(ddb.Namespace).Get(secretVolume.SecretName, metav1.GetOptions{})
	if err != nil && kerr.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	// if api.LabelDatabaseKind not exists in secret, then the secret is not created by KubeDB-Operator
	dbKind, err := meta_util.GetStringValue(secret.ObjectMeta.Labels, api.LabelDatabaseKind)
	if err != nil || dbKind != api.ResourceKindMongoDB {
		return nil
	}

	// Get object reference of dormant database
	ref, err := reference.GetReference(clientsetscheme.Scheme, ddb)
	if err != nil {
		return err
	}

	mongodbList, err := extClient.MongoDBs(ddb.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, mongodb := range mongodbList.Items {
		databaseSecret := mongodb.Spec.DatabaseSecret
		if databaseSecret != nil {
			if databaseSecret.SecretName == secretVolume.SecretName {
				secretFound = true
				break
			}
		}
	}

	if !secretFound {
		labelMap := map[string]string{
			api.LabelDatabaseKind: api.ResourceKindMongoDB,
		}
		dormantDatabaseList, err := extClient.DormantDatabases(ddb.Namespace).List(
			metav1.ListOptions{
				LabelSelector: labels.SelectorFromSet(labelMap).String(),
			},
		)
		if err != nil {
			return err
		}

		for _, ddb := range dormantDatabaseList.Items {
			if ddb.Name == ddb.Name {
				continue
			}

			databaseSecret := ddb.Spec.Origin.Spec.MongoDB.DatabaseSecret
			if databaseSecret != nil {
				if databaseSecret.SecretName == secretVolume.SecretName {
					secretFound = true
					break
				}
			}
		}
	}

	if !secretFound {
		if _, _, err := core_util.PatchSecret(client, secret, func(in *coreV1.Secret) *coreV1.Secret {
			in.ObjectMeta = core_util.EnsureOwnerReference(in.ObjectMeta, ref)
			return in
		}); err != nil {
			return err
		}
	}

	return nil
}
