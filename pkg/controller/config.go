package controller

import (
	"time"

	"github.com/appscode/go/log/golog"
	pcm "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1"
	cs "github.com/kubedb/apimachinery/client/clientset/versioned"
	amc "github.com/kubedb/apimachinery/pkg/controller"
	snapc "github.com/kubedb/apimachinery/pkg/controller/snapshot"
	"github.com/kubedb/apimachinery/pkg/eventer"
	"github.com/kubedb/mongodb/pkg/docker"
	crd_cs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var (
	AnalyticsClientID string
	EnableAnalytics   = true
	LoggerOptions     golog.Options
)

type Config struct {
	Docker            docker.Docker
	OperatorNamespace string
	GoverningService  string

	ResyncPeriod   time.Duration
	MaxNumRequeues int
	NumThreads     int

	OpsAddress string

	LoggerOptions golog.Options

	EnableAnalytics   bool
	AnalyticsClientID string
}

type OperatorConfig struct {
	Config

	ClientConfig     *rest.Config
	KubeClient       kubernetes.Interface
	APIExtKubeClient crd_cs.ApiextensionsV1beta1Interface
	DBClient         cs.Interface
	PromClient       pcm.MonitoringV1Interface
	CronController   snapc.CronControllerInterface
}

func NewOperatorConfig(clientConfig *rest.Config) *OperatorConfig {
	return &OperatorConfig{
		ClientConfig: clientConfig,
	}
}

func (c *OperatorConfig) New() (*Controller, error) {
	ctrl := &Controller{
		Controller: &amc.Controller{
			Client:           c.KubeClient,
			ExtClient:        c.DBClient.KubedbV1alpha1(),
			ApiExtKubeClient: c.APIExtKubeClient,
		},
		promClient:     c.PromClient,
		cronController: c.CronController,
		recorder:       eventer.NewEventRecorder(c.KubeClient, "MongoDB operator"),
		syncPeriod:     time.Minute * 5,
	}

	if err := ctrl.EnsureCustomResourceDefinitions(); err != nil {
		return nil, err
	}

	// ---------------------------
	// ctrl.packInformerFactory = packinformers.NewSharedInformerFactory(ctrl.PackClient, c.ResyncPeriod)
	// ctrl.setupInformers()
	// ---------------------------

	// if err := ctrl.Configure(); err != nil {
	// 	return nil, err
	// }
	return ctrl, nil
}
