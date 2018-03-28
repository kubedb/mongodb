package dormant_database

import (
	"fmt"
	"sync"

	hookapi "github.com/appscode/kubernetes-webhook-util/admission/v1beta1"
	core_util "github.com/appscode/kutil/core/v1"
	meta_util "github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	cs "github.com/kubedb/apimachinery/client/clientset/versioned"
	"github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	updUtil "github.com/kubedb/kubedb-server/pkg/admission/util"
	mgv "github.com/kubedb/mongodb/pkg/validator"
	admission "k8s.io/api/admission/v1beta1"
	coreV1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/reference"
)

type DormantDatabaseValidator struct {
	client      kubernetes.Interface
	extClient   cs.Interface
	lock        sync.RWMutex
	initialized bool
}

var _ hookapi.AdmissionHook = &DormantDatabaseValidator{}

func (a *DormantDatabaseValidator) Resource() (plural schema.GroupVersionResource, singular string) {
	return schema.GroupVersionResource{
			Group:    "admission.kubedb.com",
			Version:  "v1alpha1",
			Resource: "dormantdatabasereviews",
		},
		"dormantdatabasereview"
}

func (a *DormantDatabaseValidator) Initialize(config *rest.Config, stopCh <-chan struct{}) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.initialized = true

	var err error
	if a.client, err = kubernetes.NewForConfig(config); err != nil {
		return err
	}
	if a.extClient, err = cs.NewForConfig(config); err != nil {
		return err
	}
	return err
}

func (a *DormantDatabaseValidator) Admit(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	status := &admission.AdmissionResponse{}

	// No validation on CREATE
	if (req.Operation != admission.Update && req.Operation != admission.Delete) ||
		len(req.SubResource) != 0 ||
		req.Kind.Group != api.SchemeGroupVersion.Group ||
		req.Kind.Kind != api.ResourceKindDormantDatabase {
		status.Allowed = true
		return status
	}

	a.lock.RLock()
	defer a.lock.RUnlock()
	if !a.initialized {
		return hookapi.StatusUninitialized()
	}

	switch req.Operation {
	case admission.Delete:
		// req.Object.Raw = nil, so read from kubernetes
		obj, err := a.extClient.KubedbV1alpha1().DormantDatabases(req.Namespace).Get(req.Name, metav1.GetOptions{})
		if err != nil && !kerr.IsNotFound(err) {
			return hookapi.StatusInternalServerError(err)
		} else if kerr.IsNotFound(err) {
			break
		}

		if err := a.handleOwnerReferences(obj); err != nil {
			return hookapi.StatusInternalServerError(err)
		}
	case admission.Update:
		// validate the operation made by User
		obj, err := meta_util.UnmarshalFromJSON(req.Object.Raw, api.SchemeGroupVersion)
		if err != nil {
			return hookapi.StatusBadRequest(err)
		}
		OldObj, err := meta_util.UnmarshalFromJSON(req.OldObject.Raw, api.SchemeGroupVersion)
		if err != nil {
			return hookapi.StatusBadRequest(err)
		}
		if err := updUtil.ValidateUpdate(obj, OldObj, req.Kind.Kind); err != nil {
			return hookapi.StatusBadRequest(fmt.Errorf("%v", err))
		}
	}

	status.Allowed = true
	return status
}

func (a *DormantDatabaseValidator) handleOwnerReferences(ddb *api.DormantDatabase) error {
	if ddb.Spec.WipeOut {
		if err := a.setOwnerReferenceToObjects(ddb); err != nil {
			return err
		}
	} else {
		if err := a.removeOwnerReferenceFromObjects(ddb); err != nil {
			return err
		}
	}
	return nil
}

func (a *DormantDatabaseValidator) setOwnerReferenceToObjects(ddb *api.DormantDatabase) error {
	// Get LabelSelector for Other Components first
	dbKind, err := meta_util.GetStringValue(ddb.ObjectMeta.Labels, api.LabelDatabaseKind)
	if err != nil {
		return err
	}
	labelMap := map[string]string{
		api.LabelDatabaseName: ddb.Name,
		api.LabelDatabaseKind: dbKind,
	}
	labelSelector := labels.SelectorFromSet(labelMap)

	// Get object reference of dormant database
	ref, err := reference.GetReference(clientsetscheme.Scheme, ddb)
	if err != nil {
		return err
	}

	// Set Owner Reference of Snapshots to this Dormant Database Object
	snapshotList, err := a.extClient.KubedbV1alpha1().Snapshots(ddb.Namespace).List(
		metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		},
	)
	if err != nil {
		return err
	}
	for _, snapshot := range snapshotList.Items {
		if _, _, err := util.PatchSnapshot(a.extClient.KubedbV1alpha1(), &snapshot, func(in *api.Snapshot) *api.Snapshot {
			in.ObjectMeta = core_util.EnsureOwnerReference(in.ObjectMeta, ref)
			return in
		}); err != nil {
			return err
		}
	}

	// Set Owner Reference of PVC to this Dormant Database Object
	pvcList, err := a.client.CoreV1().PersistentVolumeClaims(ddb.Namespace).List(
		metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		},
	)
	if err != nil {
		return err
	}
	for _, pvc := range pvcList.Items {
		if _, _, err := core_util.PatchPVC(a.client, &pvc, func(in *coreV1.PersistentVolumeClaim) *coreV1.PersistentVolumeClaim {
			in.ObjectMeta = core_util.EnsureOwnerReference(in.ObjectMeta, ref)
			return in
		}); err != nil {
			return err
		}
	}

	// Set Owner Reference of Secret to this Dormant Database Object
	// KubeDB-Operator has set label-selector to only those secrets,
	// that are created by Kube-DB operator.
	return setOwnerReferenceToSecret(a.client, a.extClient, ddb, dbKind)
}

// setOwnerReferenceToSecret will set owner reference to secrets only if there is no other database or
// dormant database using this Secret.
// Yet, User can reuse those secret for other database objects.
// So make sure before deleting that nobody else is using these.
func setOwnerReferenceToSecret(client kubernetes.Interface, extClient cs.Interface, ddb *api.DormantDatabase, dbKind string) error {
	if dbKind == api.ResourceKindMemcached || dbKind == api.ResourceKindRedis {
		return nil
	}
	switch dbKind {
	case api.ResourceKindMongoDB:
		if err := mgv.SterilizeSecrets(client, extClient.KubedbV1alpha1(), ddb); err != nil {
			return err
		}
		//todo: add other cases
	}

	return nil
}

func (a *DormantDatabaseValidator) removeOwnerReferenceFromObjects(ddb *api.DormantDatabase) error {
	// First, Get LabelSelector for Other Components
	dbKind, err := meta_util.GetStringValue(ddb.ObjectMeta.Labels, api.LabelDatabaseKind)
	if err != nil {
		return err
	}
	labelMap := map[string]string{
		api.LabelDatabaseName: ddb.Name,
		api.LabelDatabaseKind: dbKind,
	}
	labelSelector := labels.SelectorFromSet(labelMap)

	// Get object reference of dormant database
	ref, err := reference.GetReference(clientsetscheme.Scheme, ddb)
	if err != nil {
		return err
	}

	// Set Owner Reference of Snapshots to this Dormant Database Object
	snapshotList, err := a.extClient.KubedbV1alpha1().Snapshots(ddb.Namespace).List(
		metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		},
	)
	if err != nil {
		return err
	}
	for _, snapshot := range snapshotList.Items {
		if _, _, err := util.PatchSnapshot(a.extClient.KubedbV1alpha1(), &snapshot, func(in *api.Snapshot) *api.Snapshot {
			in.ObjectMeta = core_util.RemoveOwnerReference(in.ObjectMeta, ref)
			return in
		}); err != nil {
			return err
		}
	}

	// Set Owner Reference of PVC to this Dormant Database Object
	pvcList, err := a.client.CoreV1().PersistentVolumeClaims(ddb.Namespace).List(
		metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		},
	)
	if err != nil {
		return err
	}
	for _, pvc := range pvcList.Items {
		if _, _, err := core_util.PatchPVC(a.client, &pvc, func(in *coreV1.PersistentVolumeClaim) *coreV1.PersistentVolumeClaim {
			in.ObjectMeta = core_util.RemoveOwnerReference(in.ObjectMeta, ref)
			return in
		}); err != nil {
			return err
		}
	}

	// Remove owner reference from Secrets
	secretVolSrc := getDatabaseSecretName(ddb, dbKind)
	if secretVolSrc == nil {
		return nil
	}

	secret, err := a.client.CoreV1().Secrets(ddb.Namespace).Get(secretVolSrc.SecretName, metav1.GetOptions{})
	if err != nil && kerr.IsNotFound(err) {
		return nil
	} else if err != nil {
		return err
	}

	if _, _, err := core_util.PatchSecret(a.client, secret, func(in *coreV1.Secret) *coreV1.Secret {
		in.ObjectMeta = core_util.RemoveOwnerReference(in.ObjectMeta, ref)
		return in
	}); err != nil {
		return err
	}
	return nil
}

func getDatabaseSecretName(ddb *api.DormantDatabase, dbKind string) *coreV1.SecretVolumeSource {
	if dbKind == api.ResourceKindMemcached || dbKind == api.ResourceKindRedis {
		return nil
	}
	switch dbKind {
	case api.ResourceKindMongoDB:
		return ddb.Spec.Origin.Spec.MongoDB.DatabaseSecret
	case api.ResourceKindMySQL:
		return ddb.Spec.Origin.Spec.MySQL.DatabaseSecret
	case api.ResourceKindPostgres:
		return ddb.Spec.Origin.Spec.Postgres.DatabaseSecret
	case api.ResourceKindElasticsearch:
		return ddb.Spec.Origin.Spec.Elasticsearch.DatabaseSecret
	}
	return nil
}
