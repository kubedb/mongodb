package server

import (
	"flag"
	"time"

	stringz "github.com/appscode/go/strings"
	v "github.com/appscode/go/version"
	"github.com/appscode/kutil/meta"
	prom "github.com/coreos/prometheus-operator/pkg/client/monitoring/v1"
	cs "github.com/kubedb/apimachinery/client/clientset/versioned"
	snapc "github.com/kubedb/apimachinery/pkg/controller/snapshot"
	"github.com/kubedb/mongodb/pkg/controller"
	"github.com/kubedb/mongodb/pkg/docker"
	"github.com/spf13/pflag"
	kext_cs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/client-go/kubernetes"
)

type ExtraOptions struct {
	Docker            docker.Docker
	OperatorNamespace string
	GoverningService  string
	QPS               float64
	Burst             int
	ResyncPeriod      time.Duration
	MaxNumRequeues    int
	NumThreads        int

	PrometheusCrdGroup string
	PrometheusCrdKinds prom.CrdKinds

	// Address to listen on for web interface and telemetry.
	OpsAddress string
}

func NewExtraOptions() *ExtraOptions {
	return &ExtraOptions{
		Docker: docker.Docker{
			Registry:    "kubedb",
			ExporterTag: "canary",
		},
		OperatorNamespace: meta.Namespace(),
		GoverningService:  "kubedb",
		ResyncPeriod:      10 * time.Minute,
		MaxNumRequeues:    5,
		NumThreads:        2,
		// ref: https://github.com/kubernetes/ingress-nginx/blob/e4d53786e771cc6bdd55f180674b79f5b692e552/pkg/ingress/controller/launch.go#L252-L259
		// High enough QPS to fit all expected use cases. QPS=0 is not set here, because client code is overriding it.
		QPS: 1e6,
		// High enough Burst to fit all expected use cases. Burst=0 is not set here, because client code is overriding it.
		Burst:              1e6,
		PrometheusCrdGroup: prom.Group,
		PrometheusCrdKinds: prom.DefaultCrdKinds,
		OpsAddress:         ":8080",
	}
}

func (s *ExtraOptions) AddGoFlags(fs *flag.FlagSet) {
	fs.StringVar(&s.Docker.Registry, "docker-registry", s.Docker.Registry, "User provided docker repository")
	fs.StringVar(&s.Docker.ExporterTag, "exporter-tag", stringz.Val(v.Version.Version, s.Docker.ExporterTag), "Tag of kubedb/operator used as exporter")
	fs.StringVar(&s.GoverningService, "governing-service", s.GoverningService, "Governing service for database statefulset")

	fs.Float64Var(&s.QPS, "qps", s.QPS, "The maximum QPS to the master from this client")
	fs.IntVar(&s.Burst, "burst", s.Burst, "The maximum burst for throttle")
	fs.DurationVar(&s.ResyncPeriod, "resync-period", s.ResyncPeriod, "If non-zero, will re-list this often. Otherwise, re-list will be delayed aslong as possible (until the upstream source closes the watch or times out.")

	fs.StringVar(&s.OpsAddress, "address", s.OpsAddress, "Address to listen on for web interface and telemetry.")

	fs.StringVar(&s.PrometheusCrdGroup, "prometheus-crd-apigroup", s.PrometheusCrdGroup, "prometheus CRD  API group name")
	fs.Var(&s.PrometheusCrdKinds, "prometheus-crd-kinds", " - EXPERIMENTAL (could be removed in future releases) - customize CRD kind names")
}

func (s *ExtraOptions) AddFlags(fs *pflag.FlagSet) {
	pfs := flag.NewFlagSet("mongodb-server", flag.ExitOnError)
	s.AddGoFlags(pfs)
	fs.AddGoFlagSet(pfs)
}

func (s *ExtraOptions) ApplyTo(cfg *controller.OperatorConfig) error {
	var err error

	cfg.Docker = s.Docker
	cfg.OperatorNamespace = s.OperatorNamespace
	cfg.GoverningService = s.GoverningService

	cfg.ClientConfig.QPS = float32(s.QPS)
	cfg.ClientConfig.Burst = s.Burst
	cfg.ResyncPeriod = s.ResyncPeriod
	cfg.MaxNumRequeues = s.MaxNumRequeues
	cfg.NumThreads = s.NumThreads

	cfg.OpsAddress = s.OpsAddress

	if cfg.KubeClient, err = kubernetes.NewForConfig(cfg.ClientConfig); err != nil {
		return err
	}
	if cfg.APIExtKubeClient, err = kext_cs.NewForConfig(cfg.ClientConfig); err != nil {
		return err
	}
	if cfg.DBClient, err = cs.NewForConfig(cfg.ClientConfig); err != nil {
		return err
	}
	if cfg.PromClient, err = prom.NewForConfig(&s.PrometheusCrdKinds, s.PrometheusCrdGroup, cfg.ClientConfig); err != nil {
		return err
	}
	cfg.CronController = snapc.NewCronController(cfg.KubeClient, cfg.DBClient.KubedbV1alpha1())

	return nil
}
