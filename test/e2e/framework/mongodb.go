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

package framework

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"kubedb.dev/apimachinery/apis/catalog/v1alpha1"
	"kubedb.dev/apimachinery/apis/kubedb"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/go/types"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	meta_util "kmodules.xyz/client-go/meta"
)

var (
	JobPvcStorageSize = "2Gi"
	DBPvcStorageSize  = "1Gi"
)

const (
	kindEviction = "Eviction"
)

func (i *Invocation) MongoDBStandalone() *api.MongoDB {
	return &api.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      rand.WithUniqSuffix("mongodb"),
			Namespace: i.namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},
		Spec: api.MongoDBSpec{
			Version: DBCatalogName,
			Storage: &core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
					},
				},
				StorageClassName: types.StringP(i.StorageClass),
			},
			SSLMode: api.SSLModeDisabled,

			TerminationPolicy: api.TerminationPolicyHalt,
		},
	}
}

func (i *Invocation) MongoDBRS() *api.MongoDB {
	dbName := rand.WithUniqSuffix("mongo-rs")
	return &api.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: i.namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},
		Spec: api.MongoDBSpec{
			Version:  DBCatalogName,
			Replicas: types.Int32P(2),
			ReplicaSet: &api.MongoDBReplicaSet{
				Name: dbName,
			},
			Storage: &core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
					},
				},
				StorageClassName: types.StringP(i.StorageClass),
			},
			TerminationPolicy: api.TerminationPolicyHalt,
		},
	}
}

func (i *Invocation) MongoDBShard() *api.MongoDB {
	dbName := rand.WithUniqSuffix("mongo-sh")
	return &api.MongoDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbName,
			Namespace: i.namespace,
			Labels: map[string]string{
				"app": i.app,
			},
		},
		Spec: api.MongoDBSpec{
			Version: DBCatalogName,
			ShardTopology: &api.MongoDBShardingTopology{
				Shard: api.MongoDBShardNode{
					Shards: 2,
					MongoDBNode: api.MongoDBNode{
						Replicas: 2,
					},
					Storage: &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
							},
						},
						StorageClassName: types.StringP(i.StorageClass),
					},
				},
				ConfigServer: api.MongoDBConfigNode{
					MongoDBNode: api.MongoDBNode{
						Replicas: 2,
					},
					Storage: &core.PersistentVolumeClaimSpec{
						Resources: core.ResourceRequirements{
							Requests: core.ResourceList{
								core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
							},
						},
						StorageClassName: types.StringP(i.StorageClass),
					},
				},
				Mongos: api.MongoDBMongosNode{
					MongoDBNode: api.MongoDBNode{
						Replicas: 2,
					},
				},
			},
			TerminationPolicy: api.TerminationPolicyHalt,
		},
	}
}

func (i *Invocation) MongoDBWithFlexibleProbeTimeout(db *api.MongoDB) *api.MongoDB {
	dbVersion, err := i.GetMongoDBVersion(DBCatalogName)
	Expect(err).NotTo(HaveOccurred())
	db.SetDefaults(dbVersion, i.topology)

	if db.Spec.ShardTopology != nil {
		db.Spec.ShardTopology.Mongos.PodTemplate.Spec.ReadinessProbe = &core.Probe{}
		db.Spec.ShardTopology.Mongos.PodTemplate.Spec.LivenessProbe = &core.Probe{}
		db.Spec.ShardTopology.Shard.PodTemplate.Spec.ReadinessProbe = &core.Probe{}
		db.Spec.ShardTopology.Shard.PodTemplate.Spec.LivenessProbe = &core.Probe{}
		db.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.ReadinessProbe = &core.Probe{}
		db.Spec.ShardTopology.ConfigServer.PodTemplate.Spec.LivenessProbe = &core.Probe{}
	} else if db.Spec.PodTemplate != nil && db.Spec.PodTemplate.Spec.ReadinessProbe != nil {
		db.Spec.PodTemplate.Spec.ReadinessProbe = &core.Probe{}
		db.Spec.PodTemplate.Spec.LivenessProbe = &core.Probe{}
	}
	return db
}

func IsRepSet(db *api.MongoDB) bool {
	return db.Spec.ReplicaSet != nil
}

// ClusterAuthModeP returns a pointer to the int32 value passed in.
func ClusterAuthModeP(v api.ClusterAuthMode) *api.ClusterAuthMode {
	return &v
}

// SSLModeP returns a pointer to the int32 value passed in.
func SSLModeP(v api.SSLMode) *api.SSLMode {
	return &v
}

func (i *Invocation) CreateMongoDB(obj *api.MongoDB) error {
	_, err := i.dbClient.KubedbV1alpha1().MongoDBs(obj.Namespace).Create(context.TODO(), obj, metav1.CreateOptions{})
	return err
}

func (f *Framework) GetMongoDB(meta metav1.ObjectMeta) (*api.MongoDB, error) {
	return f.dbClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
}

func (f *Framework) PatchMongoDB(meta metav1.ObjectMeta, transform func(*api.MongoDB) *api.MongoDB) (*api.MongoDB, error) {
	mongodb, err := f.dbClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	mongodb, _, err = util.PatchMongoDB(context.TODO(), f.dbClient.KubedbV1alpha1(), mongodb, transform, metav1.PatchOptions{})
	return mongodb, err
}

func (f *Framework) DeleteMongoDB(meta metav1.ObjectMeta) error {
	return f.dbClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}

func (f *Framework) GetMongoDBVersion(name string) (*v1alpha1.MongoDBVersion, error) {
	return f.dbClient.CatalogV1alpha1().MongoDBVersions().Get(context.TODO(), name, metav1.GetOptions{})
}

func (f *Framework) EvictPodsFromStatefulSet(meta metav1.ObjectMeta) error {
	var err error
	labelSelector := labels.Set{
		meta_util.ManagedByLabelKey: kubedb.GroupName,
		api.LabelDatabaseKind:       api.ResourceKindMongoDB,
		api.LabelDatabaseName:       meta.GetName(),
	}
	// get sts in the namespace
	stsList, err := f.kubeClient.AppsV1().StatefulSets(meta.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return err
	}
	for _, sts := range stsList.Items {
		// if PDB is not found, send error
		var pdb *policy.PodDisruptionBudget
		pdb, err = f.kubeClient.PolicyV1beta1().PodDisruptionBudgets(sts.Namespace).Get(context.TODO(), sts.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		eviction := &policy.Eviction{
			TypeMeta: metav1.TypeMeta{
				APIVersion: policy.SchemeGroupVersion.String(),
				Kind:       kindEviction,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      sts.Name,
				Namespace: sts.Namespace,
			},
			DeleteOptions: &metav1.DeleteOptions{},
		}

		if pdb.Spec.MaxUnavailable == nil {
			return fmt.Errorf("found pdb %s spec.maxUnavailable nil", pdb.Name)
		}

		// try to evict as many pod as allowed in pdb. No err should occur
		maxUnavailable := pdb.Spec.MaxUnavailable.IntValue()
		for i := 0; i < maxUnavailable; i++ {
			eviction.Name = sts.Name + "-" + strconv.Itoa(i)

			err := f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(context.TODO(), eviction)
			if err != nil {
				return err
			}
		}

		// try to evict one extra pod. TooManyRequests err should occur
		eviction.Name = sts.Name + "-" + strconv.Itoa(maxUnavailable)
		err = f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(context.TODO(), eviction)
		if kerr.IsTooManyRequests(err) {
			err = nil
		} else if err != nil {
			return err
		} else {
			return fmt.Errorf("expected pod %s/%s to be not evicted due to pdb %s", sts.Namespace, eviction.Name, pdb.Name)
		}
	}
	return err
}

func (f *Framework) EvictPodsFromDeployment(meta metav1.ObjectMeta) error {
	var err error
	deployName := meta.Name + "-mongos"
	//if PDB is not found, send error
	pdb, err := f.kubeClient.PolicyV1beta1().PodDisruptionBudgets(meta.Namespace).Get(context.TODO(), deployName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if pdb.Spec.MinAvailable == nil {
		return fmt.Errorf("found pdb %s spec.minAvailable nil", pdb.Name)
	}

	podSelector := labels.Set{
		api.MongoDBMongosLabelKey: meta.Name + "-mongos",
	}
	pods, err := f.kubeClient.CoreV1().Pods(meta.Namespace).List(context.TODO(), metav1.ListOptions{LabelSelector: podSelector.String()})
	eviction := &policy.Eviction{
		TypeMeta: metav1.TypeMeta{
			APIVersion: policy.SchemeGroupVersion.String(),
			Kind:       kindEviction,
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: meta.Namespace,
		},
		DeleteOptions: &metav1.DeleteOptions{},
	}

	// try to evict as many pods as allowed in pdb
	minAvailable := pdb.Spec.MinAvailable.IntValue()
	podCount := len(pods.Items)
	for i, pod := range pods.Items {
		eviction.Name = pod.Name
		err = f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(context.TODO(), eviction)
		if i < (podCount - minAvailable) {
			if err != nil {
				return err
			}
		} else {
			// This pod should not get evicted
			if kerr.IsTooManyRequests(err) {
				err = nil
				break
			} else if err != nil {
				return err
			} else {
				return fmt.Errorf("expected pod %s/%s to be not evicted due to pdb %s", meta.Namespace, eviction.Name, pdb.Name)
			}
		}
	}
	return err
}

func (f *Framework) EventuallyMongoDB(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.dbClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return false
				}
				Expect(err).NotTo(HaveOccurred())
			}
			return true
		},
		time.Minute*13,
		time.Second*5,
	)
}

func (f *Framework) EventuallyMongoDBPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() api.DatabasePhase {
			db, err := f.dbClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return db.Status.Phase
		},
		time.Minute*13,
		time.Second*5,
	)
}

func (f *Framework) EventuallyMongoDBRunning(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			mongodb, err := f.dbClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return mongodb.Status.Phase == api.DatabasePhaseRunning
		},
		time.Minute*13,
		time.Second*5,
	)
}

func (f *Framework) CleanMongoDB() {
	mongodbList, err := f.dbClient.KubedbV1alpha1().MongoDBs(f.namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, e := range mongodbList.Items {
		if _, _, err := util.PatchMongoDB(context.TODO(), f.dbClient.KubedbV1alpha1(), &e, func(in *api.MongoDB) *api.MongoDB {
			in.ObjectMeta.Finalizers = nil
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		}, metav1.PatchOptions{}); err != nil {
			fmt.Printf("error Patching MongoDB. error: %v", err)
		}
	}
	if err := f.dbClient.KubedbV1alpha1().MongoDBs(f.namespace).DeleteCollection(context.TODO(), meta_util.DeleteInForeground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of MongoDB. Error: %v", err)
	}
}

func (f *Framework) EventuallyWipedOut(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() error {
			labelMap := map[string]string{
				api.LabelDatabaseName: meta.Name,
				api.LabelDatabaseKind: api.ResourceKindMongoDB,
			}
			labelSelector := labels.SelectorFromSet(labelMap)

			// check if pvcs is wiped out
			pvcList, err := f.kubeClient.CoreV1().PersistentVolumeClaims(meta.Namespace).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			if err != nil {
				return err
			}
			if len(pvcList.Items) > 0 {
				fmt.Println("PVCs have not wiped out yet")
				return fmt.Errorf("PVCs have not wiped out yet")
			}

			// check if secrets are wiped out
			secretList, err := f.kubeClient.CoreV1().Secrets(meta.Namespace).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			if err != nil {
				return err
			}
			if len(secretList.Items) > 0 {
				fmt.Println("secrets have not wiped out yet")
				return fmt.Errorf("secrets have not wiped out yet")
			}

			// check if appbinds are wiped out
			appBindingList, err := f.appCatalogClient.AppBindings(meta.Namespace).List(
				context.TODO(),
				metav1.ListOptions{
					LabelSelector: labelSelector.String(),
				},
			)
			if err != nil {
				return err
			}
			if len(appBindingList.Items) > 0 {
				fmt.Println("appBindings have not wiped out yet")
				return fmt.Errorf("appBindings have not wiped out yet")
			}

			return nil
		},
		time.Minute*5,
		time.Second*5,
	)
}
