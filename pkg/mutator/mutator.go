package mutator

import (
	"fmt"

	"github.com/appscode/go/log"
	"github.com/appscode/go/types"
	mon_api "github.com/appscode/kube-mon/api"
	"github.com/appscode/kutil"
	core_util "github.com/appscode/kutil/core/v1"
	meta_util "github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	cs "github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1"
	"github.com/pkg/errors"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

// SetDefaultValues provides the defaulting that is performed in mutating stage of creating/updating a MongoDB database
func SetDefaultValues(client kubernetes.Interface, extClient cs.KubedbV1alpha1Interface, mongodb *api.MongoDB) (runtime.Object, error) {
	if mongodb.Spec.Version == "" {
		return nil, fmt.Errorf(`object 'Version' is missing in '%v'`, mongodb.Spec)
	}

	if mongodb.Spec.Replicas == nil {
		mongodb.Spec.Replicas = types.Int32P(1)
	}

	if err := setDefaultsFromDormantDB(extClient, mongodb); err != nil {
		return nil, err
	}

	// If monitoring spec is given without port,
	// set default Listening port
	setMonitoringPort(mongodb)

	return mongodb, nil
}

// setDefaultsFromDormantDB takes values from Similar Dormant Database
func setDefaultsFromDormantDB(extClient cs.KubedbV1alpha1Interface, mongodb *api.MongoDB) error {
	// Check if DormantDatabase exists or not
	dormantDb, err := extClient.DormantDatabases(mongodb.Namespace).Get(mongodb.Name, metav1.GetOptions{})
	if err != nil {
		if !kerr.IsNotFound(err) {
			return err
		}
		return nil
	}

	// Check DatabaseKind
	if value, _ := meta_util.GetStringValue(dormantDb.Labels, api.LabelDatabaseKind); value != api.ResourceKindMongoDB {
		return errors.New(fmt.Sprintf(`invalid MongoDB: "%v". Exists DormantDatabase "%v" of different Kind`, mongodb.Name, dormantDb.Name))
	}

	// Check Origin Spec
	ddbOriginSpec := dormantDb.Spec.Origin.Spec.MongoDB

	// If DatabaseSecret of new object is not given,
	// Take dormantDatabaseSecretName
	if mongodb.Spec.DatabaseSecret == nil {
		mongodb.Spec.DatabaseSecret = ddbOriginSpec.DatabaseSecret
	} else {
		ddbOriginSpec.DatabaseSecret = mongodb.Spec.DatabaseSecret
	}

	// If Monitoring Spec of new object is not given,
	// Take Monitoring Settings from Dormant
	if mongodb.Spec.Monitor == nil {
		mongodb.Spec.Monitor = ddbOriginSpec.Monitor
	} else {
		ddbOriginSpec.Monitor = mongodb.Spec.Monitor
	}

	// If Backup Scheduler of new object is not given,
	// Take Backup Scheduler Settings from Dormant
	if mongodb.Spec.BackupSchedule == nil {
		mongodb.Spec.BackupSchedule = ddbOriginSpec.BackupSchedule
	} else {
		ddbOriginSpec.BackupSchedule = mongodb.Spec.BackupSchedule
	}

	// Skip checking DoNotPause
	ddbOriginSpec.DoNotPause = mongodb.Spec.DoNotPause

	if !meta_util.Equal(ddbOriginSpec, &mongodb.Spec) {
		diff := meta_util.Diff(ddbOriginSpec, &mongodb.Spec)
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

	// Delete  Matching dormantDatabase in Controller

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
