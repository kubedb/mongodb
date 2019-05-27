package framework

import (
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	jsonTypes "github.com/appscode/go/encoding/json/types"
	"github.com/appscode/go/types"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
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
	EvictionKind = "Eviction"
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
			Version: jsonTypes.StrYo(DBCatalogName),
			Storage: &core.PersistentVolumeClaimSpec{
				Resources: core.ResourceRequirements{
					Requests: core.ResourceList{
						core.ResourceStorage: resource.MustParse(DBPvcStorageSize),
					},
				},
				StorageClassName: types.StringP(i.StorageClass),
			},
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
			Version:  jsonTypes.StrYo(DBCatalogName),
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
			Version: jsonTypes.StrYo(DBCatalogName),
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
		},
	}
}

func IsRepSet(db *api.MongoDB) bool {
	if db.Spec.ReplicaSet != nil {
		return true
	}
	return false
}

func (i *Invocation) CreateMongoDB(obj *api.MongoDB) error {
	_, err := i.extClient.KubedbV1alpha1().MongoDBs(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) GetMongoDB(meta metav1.ObjectMeta) (*api.MongoDB, error) {
	return f.extClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
}

func (f *Framework) PatchMongoDB(meta metav1.ObjectMeta, transform func(*api.MongoDB) *api.MongoDB) (*api.MongoDB, error) {
	mongodb, err := f.extClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	mongodb, _, err = util.PatchMongoDB(f.extClient.KubedbV1alpha1(), mongodb, transform)
	return mongodb, err
}

func (f *Framework) DeleteMongoDB(meta metav1.ObjectMeta) error {
	return f.extClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Delete(meta.Name, deleteInForeground())
}

func (f *Framework) EvictMongoDBStatefulSetPod(meta metav1.ObjectMeta) (bool, error) {
	var stsEvicted = true
	var evicted = 0
	var notEvicted = 0
	var err error

	labelSelector := labels.Set{
		meta_util.ManagedByLabelKey: api.GenericKey,
		api.LabelDatabaseKind:       api.ResourceKindMongoDB,
		api.LabelDatabaseName:       meta.GetName(),
	}
	//get sts in the namespace
	stsList, err := f.kubeClient.AppsV1().StatefulSets(meta.Namespace).List(metav1.ListOptions{LabelSelector: labelSelector.String()})
	if err != nil {
		return false, err
	}
	stsSize := len(stsList.Items)
	for _, sts := range stsList.Items {
		// if PDB is not found, send error
		_, err = f.kubeClient.PolicyV1beta1().PodDisruptionBudgets(sts.Namespace).Get(sts.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}

		eviction := &policy.Eviction{
			TypeMeta: metav1.TypeMeta{
				APIVersion: policy.SchemeGroupVersion.String(),
				Kind:       EvictionKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      sts.Name + "-0",
				Namespace: sts.Namespace,
			},
			DeleteOptions: &metav1.DeleteOptions{},
		}
		err = f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(eviction)
		if err == nil {
			evicted++
			stsEvicted = true
		}
		if kerr.IsTooManyRequests(err) {
			err = nil
			stsEvicted = false
			notEvicted++
		}
	}
	// ensuring result symmetry
	if evicted != stsSize && notEvicted != stsSize {
		stsEvicted = !stsEvicted
	}
	return stsEvicted, err
}

func (f *Framework) EvictMongoDBDeploymentPod(meta metav1.ObjectMeta) (bool, error) {
	var found = false
	var deployEvicted = true
	var err error
	deployName := meta.Name + "-mongos"
	//if PDB is not found, send error
	for i := 0; i < 5 && !found; i++ {
		_, err := f.kubeClient.PolicyV1beta1().PodDisruptionBudgets(meta.Namespace).Get(deployName, metav1.GetOptions{})
		if err == nil {
			found = true
		} else {
			time.Sleep(time.Second * 3)
		}
	}
	if !found {
		return false, err
	}
	podSelector := labels.Set{
		api.MongoDBMongosLabelKey: meta.Name + "-mongos",
	}
	deployedPodLists, err := f.kubeClient.CoreV1().Pods(meta.Namespace).List(metav1.ListOptions{LabelSelector: podSelector.String()})
	//Delete a pod
	for _, pod := range deployedPodLists.Items {
		eviction := &policy.Eviction{
			TypeMeta: metav1.TypeMeta{
				APIVersion: policy.SchemeGroupVersion.String(),
				Kind:       EvictionKind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      pod.Name,
				Namespace: pod.Namespace,
			},
			DeleteOptions: &metav1.DeleteOptions{},
		}
		err = f.kubeClient.PolicyV1beta1().Evictions(eviction.Namespace).Evict(eviction)
		break
	}
	if err == nil {
		deployEvicted = true
	}
	if kerr.IsTooManyRequests(err) {
		err = nil
		deployEvicted = false
	}
	return deployEvicted, err
}

func (f *Framework) EventuallyMongoDB(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.extClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return false
				}
				Expect(err).NotTo(HaveOccurred())
			}
			return true
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Framework) EventuallyMongoDBRunning(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			mongodb, err := f.extClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return mongodb.Status.Phase == api.DatabasePhaseRunning
		},
		time.Minute*10,
		time.Second*5,
	)
}

func (f *Framework) CleanMongoDB() {
	mongodbList, err := f.extClient.KubedbV1alpha1().MongoDBs(f.namespace).List(metav1.ListOptions{})
	if err != nil {
		return
	}
	for _, e := range mongodbList.Items {
		if _, _, err := util.PatchMongoDB(f.extClient.KubedbV1alpha1(), &e, func(in *api.MongoDB) *api.MongoDB {
			in.ObjectMeta.Finalizers = nil
			in.Spec.TerminationPolicy = api.TerminationPolicyWipeOut
			return in
		}); err != nil {
			fmt.Printf("error Patching MongoDB. error: %v", err)
		}
	}
	if err := f.extClient.KubedbV1alpha1().MongoDBs(f.namespace).DeleteCollection(deleteInForeground(), metav1.ListOptions{}); err != nil {
		fmt.Printf("error in deletion of MongoDB. Error: %v", err)
	}
}
