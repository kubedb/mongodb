package snapshot

import (
	"fmt"
	"sync"

	hookapi "github.com/appscode/kutil/admission/v1beta1"
	"github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	amv "github.com/kubedb/apimachinery/pkg/validator"
	"github.com/kubedb/kubedb-server/pkg/admission/util"
	admission "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type SnapshotValidator struct {
	client      kubernetes.Interface
	lock        sync.RWMutex
	initialized bool
}

var _ hookapi.AdmissionHook = &SnapshotValidator{}

func (a *SnapshotValidator) Resource() (plural schema.GroupVersionResource, singular string) {
	return schema.GroupVersionResource{
			Group:    "admission.kubedb.com",
			Version:  "v1alpha1",
			Resource: "snapshotreviews",
		},
		"snapshotreview"
}

func (a *SnapshotValidator) Initialize(config *rest.Config, stopCh <-chan struct{}) error {
	a.lock.Lock()
	defer a.lock.Unlock()

	a.initialized = true

	var err error
	a.client, err = kubernetes.NewForConfig(config)
	return err
}

func (a *SnapshotValidator) Admit(req *admission.AdmissionRequest) *admission.AdmissionResponse {
	status := &admission.AdmissionResponse{}

	if (req.Operation != admission.Create && req.Operation != admission.Update) ||
		len(req.SubResource) != 0 ||
		req.Kind.Group != api.SchemeGroupVersion.Group ||
		req.Kind.Kind != api.ResourceKindSnapshot {
		status.Allowed = true
		return status
	}

	a.lock.RLock()
	defer a.lock.RUnlock()
	if !a.initialized {
		return hookapi.StatusUninitialized()
	}

	obj, err := meta.UnmarshalFromJSON(req.Object.Raw, api.SchemeGroupVersion)
	if err != nil {
		return hookapi.StatusBadRequest(err)
	}
	if req.Operation == admission.Update && !util.IsKubeDBOperator(req.UserInfo) {
		oldObject, err := meta.UnmarshalFromJSON(req.OldObject.Raw, api.SchemeGroupVersion)
		if err != nil {
			return hookapi.StatusBadRequest(err)
		}
		if err := util.ValidateUpdate(obj, oldObject, req.Kind.Kind); err != nil {
			return hookapi.StatusBadRequest(fmt.Errorf("%v", err))
		}
	}
	if err := amv.ValidateSnapshotSpec(a.client, obj.(*api.Snapshot).Spec.SnapshotStorageSpec, req.Namespace); err != nil {
		return hookapi.StatusForbidden(err)
	}

	status.Allowed = true
	return status
}
