package admission

import (
	"net/http"
	"testing"

	types2 "github.com/appscode/go/encoding/json/types"
	"github.com/appscode/go/types"
	admission "k8s.io/api/admission/v1beta1"
	apps "k8s.io/api/apps/v1"
	authenticationV1 "k8s.io/api/authentication/v1"
	core "k8s.io/api/core/v1"
	storageV1beta1 "k8s.io/api/storage/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientSetScheme "k8s.io/client-go/kubernetes/scheme"
	"kmodules.xyz/client-go/meta"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
	catalog "kubedb.dev/apimachinery/apis/catalog/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	extFake "kubedb.dev/apimachinery/client/clientset/versioned/fake"
	"kubedb.dev/apimachinery/client/clientset/versioned/scheme"
)

func init() {
	scheme.AddToScheme(clientSetScheme.Scheme)
}

var requestKind = metaV1.GroupVersionKind{
	Group:   api.SchemeGroupVersion.Group,
	Version: api.SchemeGroupVersion.Version,
	Kind:    api.ResourceKindMongoDB,
}

func TestMongoDBValidator_Admit(t *testing.T) {
	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			//c.object.SetDefaults()

			validator := MongoDBValidator{}

			validator.initialized = true
			validator.extClient = extFake.NewSimpleClientset(
				&catalog.MongoDBVersion{
					ObjectMeta: metaV1.ObjectMeta{
						Name: "3.4",
					},
				},
			)
			validator.client = fake.NewSimpleClientset(
				&core.Secret{
					ObjectMeta: metaV1.ObjectMeta{
						Name:      "foo-auth",
						Namespace: "default",
					},
				},
				&storageV1beta1.StorageClass{
					ObjectMeta: metaV1.ObjectMeta{
						Name: "standard",
					},
				},
			)

			objJS, err := meta.MarshalToJson(&c.object, api.SchemeGroupVersion)
			if err != nil {
				panic(err)
			}
			oldObjJS, err := meta.MarshalToJson(&c.oldObject, api.SchemeGroupVersion)
			if err != nil {
				panic(err)
			}

			req := new(admission.AdmissionRequest)

			req.Kind = c.kind
			req.Name = c.objectName
			req.Namespace = c.namespace
			req.Operation = c.operation
			req.UserInfo = authenticationV1.UserInfo{}
			req.Object.Raw = objJS
			req.OldObject.Raw = oldObjJS

			if c.heatUp {
				if _, err := validator.extClient.KubedbV1alpha1().MongoDBs(c.namespace).Create(&c.object); err != nil && !kerr.IsAlreadyExists(err) {
					t.Errorf(err.Error())
				}
			}
			if c.operation == admission.Delete {
				req.Object = runtime.RawExtension{}
			}
			if c.operation != admission.Update {
				req.OldObject = runtime.RawExtension{}
			}

			response := validator.Admit(req)
			if c.result == true {
				if response.Allowed != true {
					t.Errorf("expected: 'Allowed=true'. but got response: %v", response)
				}
			} else if c.result == false {
				if response.Allowed == true || response.Result.Code == http.StatusInternalServerError {
					t.Errorf("expected: 'Allowed=false', but got response: %v", response)
				}
			}
		})
	}

}

var cases = []struct {
	testName   string
	kind       metaV1.GroupVersionKind
	objectName string
	namespace  string
	operation  admission.Operation
	object     api.MongoDB
	oldObject  api.MongoDB
	heatUp     bool
	result     bool
}{
	{"Create Valid MongoDB",
		requestKind,
		"foo",
		"default",
		admission.Create,
		sampleMongoDB(),
		api.MongoDB{},
		false,
		true,
	},
	{"Create Invalid MongoDB",
		requestKind,
		"foo",
		"default",
		admission.Create,
		getAwkwardMongoDB(),
		api.MongoDB{},
		false,
		false,
	},
	{"Edit MongoDB Spec.DatabaseSecret with Existing Secret",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editExistingSecret(sampleMongoDB()),
		sampleMongoDB(),
		false,
		true,
	},
	{"Edit MongoDB Spec.DatabaseSecret with non Existing Secret",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editNonExistingSecret(sampleMongoDB()),
		sampleMongoDB(),
		false,
		true,
	},
	{"Edit MongoDB Spec.DatabaseSecret",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editNonExistingSecret(sampleMongoDB()),
		editExistingSecret(sampleMongoDB()),
		false,
		false,
	},
	{"Edit MongoDB Sharding Prefix",
		requestKind,
		"foo",
		"default",
		admission.Update,
		sampleShardMongo(),
		editShardPrefix(sampleShardMongo()),
		false,
		false,
	},
	{"Edit Status",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editStatus(sampleMongoDB()),
		sampleMongoDB(),
		false,
		true,
	},
	{"Edit Spec.Monitor",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editSpecMonitor(sampleMongoDB()),
		sampleMongoDB(),
		false,
		true,
	},
	{"Edit Invalid Spec.Monitor",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editSpecInvalidMonitor(sampleMongoDB()),
		sampleMongoDB(),
		false,
		false,
	},
	{"Edit Spec.TerminationPolicy",
		requestKind,
		"foo",
		"default",
		admission.Update,
		pauseDatabase(sampleMongoDB()),
		sampleMongoDB(),
		false,
		true,
	},
	{"Delete MongoDB when Spec.TerminationPolicy=DoNotTerminate",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		sampleMongoDB(),
		api.MongoDB{},
		true,
		false,
	},
	{"Delete MongoDB when Spec.TerminationPolicy=Pause",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		pauseDatabase(sampleMongoDB()),
		api.MongoDB{},
		true,
		true,
	},
	{"Delete Non Existing MongoDB",
		requestKind,
		"foo",
		"default",
		admission.Delete,
		api.MongoDB{},
		api.MongoDB{},
		false,
		true,
	},
}

func sampleMongoDB() api.MongoDB {
	return api.MongoDB{
		TypeMeta: metaV1.TypeMeta{
			Kind:       api.ResourceKindMongoDB,
			APIVersion: api.SchemeGroupVersion.String(),
		},
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
			Labels: map[string]string{
				api.LabelDatabaseKind: api.ResourceKindMongoDB,
			},
		},
		Spec: api.MongoDBSpec{
			Version:     "3.4",
			Replicas:    types.Int32P(1),
			StorageType: api.StorageTypeDurable,
			Storage: &core.PersistentVolumeClaimSpec{
				StorageClassName: types.StringP("standard"),
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse("100Mi"),
					},
				},
			},
			Init: &api.InitSpec{
				ScriptSource: &api.ScriptSourceSpec{
					VolumeSource: core.VolumeSource{
						GitRepo: &core.GitRepoVolumeSource{
							Repository: "https://kubedb.dev/mongodb-init-scripts.git",
							Directory:  ".",
						},
					},
				},
			},
			UpdateStrategy: apps.StatefulSetUpdateStrategy{
				Type: apps.RollingUpdateStatefulSetStrategyType,
			},
			TerminationPolicy: api.TerminationPolicyDoNotTerminate,
		},
	}
}

func sampleShardMongo() api.MongoDB {
	return api.MongoDB{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "demo-name",
			Namespace: "demo",
			Labels: map[string]string{
				"app": "kubedb",
			},
		},
		Spec: api.MongoDBSpec{
			Version: types2.StrYo("3.6-v2"),
			ShardTopology: &api.MongoDBShardingTopology{
				Shard: api.MongoDBShardNode{
					Shards: 3,
					MongoDBNode: api.MongoDBNode{
						Replicas: 3,
					},
					Storage: &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
						StorageClassName: types.StringP("standard"),
					},
				},
				ConfigServer: api.MongoDBConfigNode{
					MongoDBNode: api.MongoDBNode{
						Replicas: 3,
					},
					Storage: &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse("1Gi"),
							},
						},
						StorageClassName: types.StringP("standard"),
					},
				},
				Mongos: api.MongoDBMongosNode{
					MongoDBNode: api.MongoDBNode{
						Replicas: 2,
					},
				},
			},
		},
	}
}

func getAwkwardMongoDB() api.MongoDB {
	mongodb := sampleMongoDB()
	mongodb.Spec.Version = "3.0"
	return mongodb
}

func editExistingSecret(old api.MongoDB) api.MongoDB {
	old.Spec.DatabaseSecret = &core.SecretVolumeSource{
		SecretName: "foo-auth",
	}
	return old
}

func editNonExistingSecret(old api.MongoDB) api.MongoDB {
	old.Spec.DatabaseSecret = &core.SecretVolumeSource{
		SecretName: "foo-auth-fused",
	}
	return old
}

func editStatus(old api.MongoDB) api.MongoDB {
	old.Status = api.MongoDBStatus{
		Phase: api.DatabasePhaseCreating,
	}
	return old
}

func editSpecMonitor(old api.MongoDB) api.MongoDB {
	old.Spec.Monitor = &mona.AgentSpec{
		Agent: mona.AgentPrometheusBuiltin,
		Prometheus: &mona.PrometheusSpec{
			Port: 1289,
		},
	}
	return old
}

// should be failed because more fields required for COreOS Monitoring
func editSpecInvalidMonitor(old api.MongoDB) api.MongoDB {
	old.Spec.Monitor = &mona.AgentSpec{
		Agent: mona.AgentCoreOSPrometheus,
	}
	return old
}

func pauseDatabase(old api.MongoDB) api.MongoDB {
	old.Spec.TerminationPolicy = api.TerminationPolicyPause
	return old
}

func editShardPrefix(old api.MongoDB) api.MongoDB {
	old.Spec.ShardTopology.Shard.Prefix = "demo-prefix"
	return old
}
