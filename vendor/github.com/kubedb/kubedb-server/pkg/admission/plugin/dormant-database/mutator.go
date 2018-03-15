package dormant_database

import (
	"sync"

	hookapi "github.com/appscode/kutil/admission/api"
	core_util "github.com/appscode/kutil/core/v1"
	meta_util "github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	cs "github.com/kubedb/apimachinery/client/clientset/versioned"
	"github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	mgm "github.com/kubedb/mongodb/pkg/mutator"
	admission "k8s.io/api/admission/v1beta1"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/reference"
)

type DormantDatabaseMutator struct {
	client      kubernetes.Interface
	extClient   cs.Interface
	lock        sync.RWMutex
	initialized bool
}

var _ hookapi.AdmissionHook = &DormantDatabaseMutator{}

func (a *DormantDatabaseMutator) Resource() (plural schema.GroupVersionResource, singular string) {
	return schema.GroupVersionResource{
			Group:    "admission.kubedb.com",
			Version:  "v1alpha1",
			Resource: "dormantdatabasemutates",
		},
		"dormantdatabasemutate"
}

func (a *DormantDatabaseMutator) Initialize(config *rest.Config, stopCh <-chan struct{}) error {
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

func (a *DormantDatabaseMutator) Admit(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	status := &admission.AdmissionResponse{}

	if (req.Operation != admission.Create && req.Operation != admission.Update && req.Operation != admission.Delete) ||
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
		//obj, err := a.extClient.KubedbV1alpha1().DormantDatabases(req.Namespace).Get(req.Name, metav1.GetOptions{})
		//if err != nil && !kerr.IsNotFound(err) {
		//	return hookapi.StatusInternalServerError(err)
		//}

	default:
		obj, err := meta_util.UnmarshalToJSON(req.Object.Raw, api.SchemeGroupVersion)
		if err != nil {
			return hookapi.StatusBadRequest(err)
		}
		mongoMod, err := mgm.OnCreate(a.client, a.extClient.KubedbV1alpha1(), *obj.(*api.MongoDB))
		if err != nil {
			return hookapi.StatusForbidden(err)
		} else if mongoMod != nil {
			patch, err := meta_util.CreateJSONPatch(obj, mongoMod)
			if err != nil {
				return hookapi.StatusInternalServerError(err)
			}
			status.Patch = patch
			patchType := admission.PatchTypeJSONPatch
			status.PatchType = &patchType
		}
	}
	status.Allowed = true
	return status
}

func (a *DormantDatabaseMutator) onDelete(ddb *api.DormantDatabase) error {
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
	a.setOwnerReferenceToSecret(ddb, dbKind)

	return nil
}

// setOwnerReferenceToSecret will set owner reference to secrets only if there is no other database or
// dormant database using this Secret.
func (a *DormantDatabaseMutator) setOwnerReferenceToSecret(ddb *api.DormantDatabase, dbKind string) error {
	if dbKind == api.ResourceKindMemcached || dbKind == api.ResourceKindRedis {
		return nil
	}
	switch dbKind {
	case api.ResourceKindMongoDB:
		if err := mgm.DeleteSecret(a.client,a.extClient.KubedbV1alpha1(),ddb); err!=nil {
			return err
		}
	}

	return nil
}
