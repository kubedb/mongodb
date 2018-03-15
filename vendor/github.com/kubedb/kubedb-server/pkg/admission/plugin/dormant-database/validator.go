package dormant_database

import (
	"sync"

	hookapi "github.com/appscode/kutil/admission/api"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	cs "github.com/kubedb/apimachinery/client/clientset/versioned"
	admission "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

	//switch req.Operation {
	//case admission.Delete:
	//	// validate the operation made by User
	//	if !util.IsKubeDBOperator(req.UserInfo) {
	//		// req.Object.Raw = nil, so read from kubernetes
	//		obj, err := a.extClient.KubedbV1alpha1().DormantDatabases(req.Namespace).Get(req.Name, metav1.GetOptions{})
	//		if err != nil && !kerr.IsNotFound(err) {
	//			return hookapi.StatusInternalServerError(err)
	//		} else if err == nil && obj.Status.Phase != api.DormantDatabasePhaseWipedOut {
	//			return hookapi.StatusBadRequest(fmt.Errorf(`dormant_database "%s" is not wipedOut. `, req.Name))
	//		}
	//	}
	//case admission.Create:
	//	// validate the operation made by User
	//	if !util.IsKubeDBOperator(req.UserInfo) {
	//		return hookapi.StatusBadRequest(fmt.Errorf(`user can't create object of Kind dormantdatabase`))
	//	}
	//case admission.Update:
	//	// validate the operation made by User
	//	if !util.IsKubeDBOperator(req.UserInfo) {
	//		obj, err := meta_util.UnmarshalToJSON(req.Object.Raw, api.SchemeGroupVersion)
	//		if err != nil {
	//			return hookapi.StatusBadRequest(err)
	//		}
	//		OldObj, err := meta_util.UnmarshalToJSON(req.OldObject.Raw, api.SchemeGroupVersion)
	//		if err != nil {
	//			return hookapi.StatusBadRequest(err)
	//		}
	//		if err := util.ValidateUpdate(obj, OldObj, req.Kind.Kind); err != nil {
	//			return hookapi.StatusBadRequest(fmt.Errorf("%v", err))
	//		}
	//	}
	//}

	status.Allowed = true
	return status
}
