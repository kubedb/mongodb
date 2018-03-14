package mutator

import (
	"fmt"

	"github.com/appscode/go/types"
	mon_api "github.com/appscode/kube-mon/api"
	"github.com/appscode/kutil"
	core_util "github.com/appscode/kutil/core/v1"
	meta_util "github.com/appscode/kutil/meta"
	"github.com/cloudflare/cfssl/log"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	cs "github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1"
	amv "github.com/kubedb/apimachinery/pkg/validator"
	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

// OnCreate provides the defaulting that is performed in mutating stage of creating/updating a MongoDB database
//
// Major Tasks:
// - Take Defaults from Dormant Database
// - Set default  values to rest of the fields
// - Remove Dormant Database Finalizer and set Spec.WipeOut to false
// - Delete Dormant Database
// - Finalizer Not Needed for MongoDB object
// N.B.: Delete dormant database at the last stage of ValidatingWebhook
func OnCreate(client kubernetes.Interface, extClient cs.KubedbV1alpha1Interface, mongodb api.MongoDB) (runtime.Object, error) {
	if mongodb.Spec.Version == "" {
		return nil, fmt.Errorf(`object 'Version' is missing in '%v'`, mongodb.Spec)
	}

	if mongodb.Spec.Replicas == nil || *mongodb.Spec.Replicas != 1 {
		mongodb.Spec.Replicas = types.Int32P(1)
	}

	if mongodb.Spec.Storage != nil {
		var err error
		if err = amv.ValidateStorage(client, mongodb.Spec.Storage); err != nil {
			return nil, err
		}
	}

	//databaseSecret := mongodb.Spec.DatabaseSecret
	//if databaseSecret != nil {
	//	if _, err := client.CoreV1().Secrets(mongodb.Namespace).Get(databaseSecret.SecretName, metav1.GetOptions{}); err != nil {
	//		return nil, err
	//	}
	//}

	backupScheduleSpec := mongodb.Spec.BackupSchedule
	if backupScheduleSpec != nil {
		if err := amv.ValidateBackupSchedule(client, backupScheduleSpec, mongodb.Namespace); err != nil {
			return nil, err
		}
	}

	monitorSpec := mongodb.Spec.Monitor
	if monitorSpec != nil {
		if err := amv.ValidateMonitorSpec(monitorSpec); err != nil {
			return nil, err
		}
	}

	if err := resembleDormantDatabase(extClient, &mongodb); err != nil {
		return nil, err
	}

	prepareMongoDB(&mongodb)

	return &mongodb, nil
}

// OnCreate provides the defaulting that is performed in mutating stage of creating/updating a MongoDB database
//
// Major Tasks:
// - Create Dormant Database with Finalizer
// Let kubernetes Garbage Collect of StatefulSets, Service
func OnDelete(client kubernetes.Interface, extClient cs.KubedbV1alpha1Interface, mongodb api.MongoDB) (runtime.Object, error) {
	ddb := getDormantDatabase(&mongodb)
	if _, err := extClient.DormantDatabases(ddb.Namespace).Create(ddb); err != nil {
		return nil, err
	}

	return &mongodb, nil
}

func resembleDormantDatabase(extClient cs.KubedbV1alpha1Interface, mongodb *api.MongoDB) error {
	// Check if DormantDatabase exists or not
	dormantDb, err := extClient.DormantDatabases(mongodb.Namespace).Get(mongodb.Name, metav1.GetOptions{})
	if err != nil {
		if !kerr.IsNotFound(err) {
			return err
		}
		return nil
	}

	// Check DatabaseKind
	if dormantDb.Labels[api.LabelDatabaseKind] != api.ResourceKindMongoDB {
		return errors.New(fmt.Sprintf(`invalid MongoDB: "%v". Exists DormantDatabase "%v" of different Kind`, mongodb.Name, dormantDb.Name))
	}

	// Check Origin Spec
	drmnOriginSpec := dormantDb.Spec.Origin.Spec.MongoDB
	originalSpec := mongodb.Spec

	// If DatabaseSecret of new object is not given,
	// Take dormantDatabaseSecretName
	if originalSpec.DatabaseSecret == nil {
		originalSpec.DatabaseSecret = drmnOriginSpec.DatabaseSecret
	} else {
		drmnOriginSpec.DatabaseSecret = originalSpec.DatabaseSecret
	}

	// Skip checking doNotPause
	drmnOriginSpec.DoNotPause = originalSpec.DoNotPause

	// Skip checking Monitoring
	drmnOriginSpec.Monitor = originalSpec.Monitor

	// Skip Checking BackUP Scheduler
	drmnOriginSpec.BackupSchedule = originalSpec.BackupSchedule

	if !meta_util.Equal(drmnOriginSpec, &originalSpec) {
		diff := meta_util.Diff(drmnOriginSpec, &originalSpec)
		log.Errorf("mongodb spec mismatches with OriginSpec in DormantDatabases. Diff: %v", diff)
		return errors.New(fmt.Sprintf("mongodb spec mismatches with OriginSpec in DormantDatabases. Diff: %v", diff))
	}

	if _, err := meta_util.GetString(mongodb.Annotations, api.AnnotationInitialized); err == kutil.ErrNotFound &&
		mongodb.Spec.Init != nil &&
		mongodb.Spec.Init.SnapshotSource != nil {
		mongodb.Annotations = core_util.UpsertMap(mongodb.Annotations, map[string]string{
			api.AnnotationInitialized: "",
		})
	}

	// Delete  Matching dormantDatabase after checking ValidatingWebhook

	return nil
}

func prepareMongoDB(mongodb *api.MongoDB) error {

	// Set Default DatabaseSecretName
	if mongodb.Spec.DatabaseSecret == nil {
		mongodb.Spec.DatabaseSecret = &core.SecretVolumeSource{
			SecretName: fmt.Sprintf("%v-auth", mongodb.Name),
		}
	}

	// If monitoring spec is given without port,
	// set default Listening port
	setMonitoringPort(mongodb)

	return nil
}

// Assign Default Monitoring Port if MonitoringSpec Exists
// and the AgentVendor is Prometheus.
func setMonitoringPort(mongodb *api.MongoDB) {
	if mongodb.Spec.Monitor != nil &&
		mongodb.GetMonitoringVendor() == mon_api.VendorPrometheus {
		if mongodb.Spec.Monitor.Prometheus == nil {
			mongodb.Spec.Monitor.Prometheus = &mon_api.PrometheusSpec{}
		}
		if mongodb.Spec.Monitor.Prometheus.Port == 0 {
			mongodb.Spec.Monitor.Prometheus.Port = api.PrometheusExporterPortNumber
		}
	}
}

func getDormantDatabase(mongodb *api.MongoDB) *api.DormantDatabase {
	return &api.DormantDatabase{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mongodb.Name,
			Namespace: mongodb.Namespace,
			Labels: map[string]string{
				api.LabelDatabaseKind: api.ResourceKindMongoDB,
			},
			Finalizers: []string{api.GenericKey},
		},
		Spec: api.DormantDatabaseSpec{
			Origin: api.Origin{
				ObjectMeta: metav1.ObjectMeta{
					Name:        mongodb.Name,
					Namespace:   mongodb.Namespace,
					Labels:      mongodb.Labels,
					Annotations: mongodb.Annotations,
				},
				Spec: api.OriginSpec{
					MongoDB: &mongodb.Spec,
				},
			},
		},
	}
}
