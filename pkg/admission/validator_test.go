/*
Copyright AppsCode Inc. and Contributors

Licensed under the AppsCode Community License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/AppsCode-Community-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package admission

import (
	"context"
	"net/http"
	"testing"

	catalog "kubedb.dev/apimachinery/apis/catalog/v1alpha1"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"
	extFake "kubedb.dev/apimachinery/client/clientset/versioned/fake"
	"kubedb.dev/apimachinery/client/clientset/versioned/scheme"

	"gomodules.xyz/pointer"
	admission "k8s.io/api/admission/v1beta1"
	authenticationV1 "k8s.io/api/authentication/v1"
	core "k8s.io/api/core/v1"
	storageV1beta1 "k8s.io/api/storage/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientSetScheme "k8s.io/client-go/kubernetes/scheme"
	core_util "kmodules.xyz/client-go/core/v1"
	meta_util "kmodules.xyz/client-go/meta"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
)

func init() {
	utilruntime.Must(scheme.AddToScheme(clientSetScheme.Scheme))
}

var testTopology = &core_util.Topology{
	Regions: map[string][]string{
		"us-east-1": {"us-east-1a", "us-east-1b", "us-east-1c"},
	},
	TotalNodes: 100,
	InstanceTypes: map[string]int{
		"n1-standard-4": 100,
	},
	LabelZone:         core.LabelZoneFailureDomain,
	LabelRegion:       core.LabelZoneRegion,
	LabelInstanceType: core.LabelInstanceType,
}

var requestKind = metaV1.GroupVersionKind{
	Group:   api.SchemeGroupVersion.Group,
	Version: api.SchemeGroupVersion.Version,
	Kind:    api.ResourceKindMongoDB,
}

func TestMongoDBValidator_Admit(t *testing.T) {
	for _, c := range cases {
		t.Run(c.testName, func(t *testing.T) {
			validator := MongoDBValidator{
				ClusterTopology: testTopology,
			}

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

			objJS, err := meta_util.MarshalToJson(&c.object, api.SchemeGroupVersion)
			if err != nil {
				panic(err)
			}
			oldObjJS, err := meta_util.MarshalToJson(&c.oldObject, api.SchemeGroupVersion)
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
				if _, err := validator.extClient.KubedbV1alpha2().MongoDBs(c.namespace).Create(context.TODO(), &c.object, metaV1.CreateOptions{}); err != nil && !kerr.IsAlreadyExists(err) {
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
	{"Edit MongoDB Spec.AuthSecret with Existing Secret",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editExistingSecret(sampleMongoDB()),
		sampleMongoDB(),
		false,
		true,
	},
	{"Edit MongoDB Spec.AuthSecret with non Existing Secret",
		requestKind,
		"foo",
		"default",
		admission.Update,
		editNonExistingSecret(sampleMongoDB()),
		sampleMongoDB(),
		false,
		true,
	},
	{"Edit MongoDB Spec.AuthSecret",
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
		haltDatabase(sampleMongoDB()),
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
		haltDatabase(sampleMongoDB()),
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
	{"Edit spec.Init before provisioning complete",
		requestKind,
		"foo",
		"default",
		admission.Update,
		updateInit(sampleMongoDB()),
		sampleMongoDB(),
		true,
		true,
	},
	{"Edit spec.Init after provisioning complete",
		requestKind,
		"foo",
		"default",
		admission.Update,
		updateInit(completeInitialization(sampleMongoDB())),
		completeInitialization(sampleMongoDB()),
		true,
		false,
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
				meta_util.NameLabelKey: api.MongoDB{}.ResourceFQN(),
			},
		},
		Spec: api.MongoDBSpec{
			Version:     "3.4",
			Replicas:    pointer.Int32P(1),
			StorageType: api.StorageTypeDurable,
			Storage: &core.PersistentVolumeClaimSpec{
				StorageClassName: pointer.StringP("standard"),
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse("100Mi"),
					},
				},
			},
			Init: &api.InitSpec{
				WaitForInitialRestore: true,
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
			Version: "3.4",
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
						StorageClassName: pointer.StringP("standard"),
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
						StorageClassName: pointer.StringP("standard"),
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
	old.Spec.AuthSecret = &core.LocalObjectReference{
		Name: "foo-auth",
	}
	return old
}

func editNonExistingSecret(old api.MongoDB) api.MongoDB {
	old.Spec.AuthSecret = &core.LocalObjectReference{
		Name: "foo-auth-fused",
	}
	return old
}

func editStatus(old api.MongoDB) api.MongoDB {
	old.Status = api.MongoDBStatus{
		Phase: api.DatabasePhaseReady,
	}
	return old
}

func editSpecMonitor(old api.MongoDB) api.MongoDB {
	old.Spec.Monitor = &mona.AgentSpec{
		Agent: mona.AgentPrometheusBuiltin,
		Prometheus: &mona.PrometheusSpec{
			Exporter: mona.PrometheusExporterSpec{
				Port: 1289,
			},
		},
	}
	return old
}

// should be failed because more fields required for COreOS Monitoring
func editSpecInvalidMonitor(old api.MongoDB) api.MongoDB {
	old.Spec.Monitor = &mona.AgentSpec{
		Agent: mona.AgentPrometheusOperator,
	}
	return old
}

func haltDatabase(old api.MongoDB) api.MongoDB {
	old.Spec.TerminationPolicy = api.TerminationPolicyHalt
	return old
}

func editShardPrefix(old api.MongoDB) api.MongoDB {
	old.Spec.ShardTopology.Shard.Prefix = "demo-prefix"
	return old
}

func completeInitialization(old api.MongoDB) api.MongoDB {
	old.Spec.Init.Initialized = true
	return old
}

func updateInit(old api.MongoDB) api.MongoDB {
	old.Spec.Init.WaitForInitialRestore = false
	return old
}
