package dormantdatabase

import (
	crdutils "github.com/appscode/kutil/apiextensions/v1beta1"
	"github.com/appscode/kutil/tools/queue"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	api_listers "github.com/kubedb/apimachinery/client/listers/kubedb/v1alpha1"
	amc "github.com/kubedb/apimachinery/pkg/controller"
	"github.com/kubedb/apimachinery/pkg/eventer"
	crd_api "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

type Controller struct {
	*amc.Controller
	amc.Config
	// Deleter interface
	deleter amc.Deleter
	// ListerWatcher
	labelMap map[string]string
	// Event Recorder
	recorder record.EventRecorder
	// DormantDatabase
	ddbQueue    *queue.Worker
	ddbInformer cache.SharedIndexInformer
	ddbLister   api_listers.DormantDatabaseLister
}

// NewController creates a new DormantDatabase Controller
func NewController(
	controller *amc.Controller,
	deleter amc.Deleter,
	config amc.Config,
	labelmap map[string]string,
) *Controller {
	// return new DormantDatabase Controller
	return &Controller{
		Controller: controller,
		deleter:    deleter,
		Config:     config,
		labelMap:   labelmap,
		recorder:   eventer.NewEventRecorder(controller.Client, "DormantDatabase Controller"),
	}
}

func (c *Controller) EnsureCustomResourceDefinitions() error {
	crd := []*crd_api.CustomResourceDefinition{
		api.DormantDatabase{}.CustomResourceDefinition(),
	}
	return crdutils.RegisterCRDs(c.ApiExtKubeClient, crd)
}

func (c *Controller) InitDormantDatabaseWatcher() *queue.Worker {
	c.initWatcher()
	return c.ddbQueue
}
