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
	"strings"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha2"

	"github.com/appscode/go/log"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kmodules.xyz/client-go/tools/portforward"
)

type KubedbTable struct {
	FirstName string
	LastName  string
}

func (f *Framework) ForwardPort(meta metav1.ObjectMeta, clientPodName string) (*portforward.Tunnel, error) {
	tunnel := portforward.NewTunnel(
		f.kubeClient.CoreV1().RESTClient(),
		f.restConfig,
		meta.Namespace,
		clientPodName,
		27017,
	)

	if err := tunnel.ForwardPort(); err != nil {
		return nil, err
	}
	return tunnel, nil
}

func (f *Framework) GetMongoDBClient(meta metav1.ObjectMeta, tunnel *portforward.Tunnel, isReplSet ...bool) (*options.ClientOptions, error) {
	mongodb, err := f.GetMongoDB(meta)
	if err != nil {
		return nil, err
	}

	user := "root"
	pass, err := f.GetMongoDBRootPassword(mongodb)
	if err != nil {
		return nil, err
	}

	clientOpts := options.Client().ApplyURI(fmt.Sprintf("mongodb://%s:%s@127.0.0.1:%v", user, pass, tunnel.Local))
	if mongodb.Spec.SSLMode == api.SSLModeRequireSSL {
		if err := f.GetSSLCertificate(meta); err != nil {
			return nil, err
		}
		clientOpts = options.Client().ApplyURI(fmt.Sprintf("mongodb://%s:%s@localhost:%v/?ssl=true&sslclientcertificatekeyfile=/tmp/mongodb/%v&&sslcertificateauthorityfile=/tmp/mongodb/%v", user, pass, tunnel.Local, api.MongoPemFileName, api.TLSCACertFileName))
	}

	if (len(isReplSet) > 0 && isReplSet[0]) || IsRepSet(mongodb) {
		clientOpts.SetDirect(true)
	}
	return clientOpts, nil
}

func (f *Framework) ConnectAndPing(meta metav1.ObjectMeta, clientPodName string, isReplSet ...bool) (*mongo.Client, *portforward.Tunnel, error) {
	tunnel, err := f.ForwardPort(meta, clientPodName)
	if err != nil {
		return nil, nil, err
	}

	clientOpts, err := f.GetMongoDBClient(meta, tunnel, isReplSet...)
	if err != nil {
		return nil, nil, err
	}

	client, err := mongo.Connect(context.Background(), clientOpts)
	if err != nil {
		return nil, nil, err
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		return nil, nil, err
	}
	return client, tunnel, err
}

func (f *Framework) GetMongosPodName(meta metav1.ObjectMeta) (string, error) {
	pods, err := f.kubeClient.CoreV1().Pods(meta.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, fmt.Sprintf("%v-mongos", meta.Name)) {
			return pod.Name, nil
		}
	}
	return "", fmt.Errorf("no pod found for mongodb: %s", meta.Name)
}

func (f *Framework) GetReplicaMasterNode(meta metav1.ObjectMeta, nodeName string, replicaNumber *int32) (string, error) {
	if replicaNumber == nil {
		return "", fmt.Errorf("replica is zero")
	}

	fn := func(clientPodName string) (bool, error) {
		client, tunnel, err := f.ConnectAndPing(meta, clientPodName, true)
		if err != nil {
			return false, err
		}
		defer tunnel.Close()

		res := make(map[string]interface{})
		if err := client.Database("admin").RunCommand(context.Background(), bson.D{{Key: "isMaster", Value: "1"}}).Decode(&res); err != nil {
			return false, err
		}

		if val, ok := res["ismaster"]; ok && val == true {
			return true, nil
		}
		return false, fmt.Errorf("%v not master node", clientPodName)
	}

	// For MongoDB ReplicaSet, Find out the primary instance.
	// Extract information `IsMaster: true` from the component's status.
	for i := int32(0); i <= *replicaNumber; i++ {
		clientPodName := fmt.Sprintf("%v-%d", nodeName, i)
		var isMaster bool
		isMaster, err := fn(clientPodName)
		if err == nil && isMaster {
			return clientPodName, nil
		}
	}
	return "", fmt.Errorf("no primary node")
}

func (f *Framework) GetPrimaryInstance(meta metav1.ObjectMeta) (string, error) {
	mongodb, err := f.GetMongoDB(meta)
	if err != nil {
		return "", err
	}

	if mongodb.Spec.ReplicaSet == nil && mongodb.Spec.ShardTopology == nil {
		return fmt.Sprintf("%v-0", mongodb.Name), nil
	}

	if mongodb.Spec.ShardTopology != nil {
		return f.GetMongosPodName(meta)
	}

	return f.GetReplicaMasterNode(meta, mongodb.RepSetName(), mongodb.Spec.Replicas)
}

func (f *Framework) EventuallyPingMongo(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			podName, err := f.GetPrimaryInstance(meta)
			if err != nil {
				log.Errorln("GetPrimaryInstance error:", err)
				return false
			}

			_, tunnel, err := f.ConnectAndPing(meta, podName)
			if err != nil {
				log.Errorln("Failed to ConnectAndPing. Reason: ", err)
				return false
			}
			defer tunnel.Close()
			return true
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyInsertDocument(meta metav1.ObjectMeta, dbName string, collectionCount int) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			podName, err := f.GetPrimaryInstance(meta)
			if err != nil {
				log.Errorln("GetPrimaryInstance error:", err)
				return false, err
			}

			client, tunnel, err := f.ConnectAndPing(meta, podName)
			if err != nil {
				log.Errorln("Failed to ConnectAndPing. Reason: ", err)
				return false, err
			}
			defer tunnel.Close()

			person := &KubedbTable{
				FirstName: "kubernetes",
				LastName:  "database",
			}

			if _, err := client.Database(dbName).Collection("people").InsertOne(context.Background(), person); err != nil {
				log.Errorln("creation error:", err)
				return false, err
			}

			// above one is 0th element
			for i := 1; i < collectionCount; i++ {

				person := &KubedbTable{
					FirstName: fmt.Sprintf("kubernetes-%03d", i),
					LastName:  fmt.Sprintf("database-%03d", i),
				}

				if _, err := client.Database(dbName).Collection(fmt.Sprintf("people-%03d", i)).InsertOne(context.Background(), person); err != nil {
					log.Errorln("creation error:", err)
					return false, err
				}
			}

			return true, nil
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyDocumentExists(meta metav1.ObjectMeta, dbName string, collectionCount int) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			podName, err := f.GetPrimaryInstance(meta)
			if err != nil {
				log.Errorln("GetPrimaryInstance error:", err)
				return false, err
			}

			client, tunnel, err := f.ConnectAndPing(meta, podName)
			if err != nil {
				log.Errorln("Failed to ConnectAndPing. Reason: ", err)
				return false, err
			}
			defer tunnel.Close()

			expected := &KubedbTable{
				FirstName: "kubernetes",
				LastName:  "database",
			}
			person := &KubedbTable{}

			if er := client.Database(dbName).Collection("people").FindOne(context.Background(), bson.M{"firstname": expected.FirstName}).Decode(&person); er != nil || person == nil || person.LastName != expected.LastName {
				log.Errorln("checking error:", er)
				return false, er
			}

			// above one is 0th element
			for i := 1; i < collectionCount; i++ {
				expected := &KubedbTable{
					FirstName: fmt.Sprintf("kubernetes-%03d", i),
					LastName:  fmt.Sprintf("database-%03d", i),
				}
				person := &KubedbTable{}

				if er := client.Database(dbName).Collection(fmt.Sprintf("people-%03d", i)).FindOne(context.Background(), bson.M{"firstname": expected.FirstName}).Decode(&person); er != nil || person == nil || person.LastName != expected.LastName {
					log.Errorln("checking error:", er)
					return false, er
				}
			}
			return true, nil
		},
		time.Minute*5,
		time.Second*5,
	)
}

// EventuallyEnableSharding enables sharding of a database. Call this only when spec.shardTopology is set.
func (f *Framework) EventuallyEnableSharding(meta metav1.ObjectMeta, dbName string) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			podName, err := f.GetPrimaryInstance(meta)
			if err != nil {
				log.Errorln("GetPrimaryInstance error:", err)
				return false, err
			}

			client, tunnel, err := f.ConnectAndPing(meta, podName, false)
			if err != nil {
				log.Errorln("Failed to ConnectAndPing. Reason: ", err)
				return false, err
			}
			defer tunnel.Close()

			singleRes := client.Database("admin").RunCommand(context.Background(), bson.D{{Key: "enableSharding", Value: dbName}})
			if singleRes.Err() != nil {
				log.Errorln("RunCommand enableSharding error:", err)
				return false, err
			}

			// Now shard collection
			singleRes = client.Database("admin").RunCommand(context.Background(), bson.D{{Key: "shardCollection", Value: fmt.Sprintf("%v.public", dbName)}, {Key: "key", Value: bson.M{"firstname": 1}}})
			if singleRes.Err() != nil {
				log.Errorln("RunCommand shardCollection error:", err)
				return false, err
			}

			return true, nil
		},
		time.Minute*5,
		time.Second*5,
	)
}

// EventuallyCollectionPartitioned checks if a database is partitioned or not. Call this only when spec.shardTopology is set.
func (f *Framework) EventuallyCollectionPartitioned(meta metav1.ObjectMeta, dbName string) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			podName, err := f.GetPrimaryInstance(meta)
			if err != nil {
				log.Errorln("GetPrimaryInstance error:", err)
				return false, err
			}

			client, tunnel, err := f.ConnectAndPing(meta, podName, false)
			if err != nil {
				log.Errorln("Failed to ConnectAndPing. Reason: ", err)
				return false, err
			}
			defer tunnel.Close()

			res := make(map[string]interface{})
			err = client.Database("config").Collection("databases").FindOne(context.TODO(), bson.D{{Key: "_id", Value: dbName}}).Decode(&res)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					log.Infoln("No document in config.databases:", err)
					return false, nil
				}
				log.Errorln("Query error:", err)
				return false, err
			}

			val, ok := res["partitioned"]
			if ok && val == true {
				return true, nil
			}
			log.Errorln("db", dbName, "is not partitioned. Got partitioned:", val)
			return false, nil
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) getMaxIncomingConnections(meta metav1.ObjectMeta, podName string, isRepSet bool) (int32, error) {
	client, tunnel, err := f.ConnectAndPing(meta, podName, isRepSet)
	if err != nil {
		return 0, fmt.Errorf("failed to ConnectAndPing. Reason: %v", err)
	}
	defer tunnel.Close()

	res := make(map[string]interface{})
	err = client.Database("admin").RunCommand(context.Background(), bson.D{{Key: "getCmdLineOpts", Value: 1}}).Decode(&res)
	if err != nil {
		log.Errorln("RunCommand getCmdLineOpts error:", err)
		return 0, err
	}

	res, ok := res["parsed"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("can't get 'parsed' value")
	}

	res, ok = res["net"].(map[string]interface{})
	if !ok {
		return 0, fmt.Errorf("can't get 'parsed.net' value")
	}

	val, ok := res["maxIncomingConnections"].(int32)
	if ok {
		return val, nil
	}

	return 0, fmt.Errorf("unable to get maxIncomingConnections")
}

func (f *Framework) EventuallyMaxIncomingConnections(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() (int32, error) {
			mongodb, err := f.GetMongoDB(meta)
			if err != nil {
				return 0, err
			}
			if mongodb.Spec.ShardTopology == nil {
				podName, err := f.GetPrimaryInstance(meta)
				if err != nil {
					log.Errorln("GetPrimaryInstance error:", err)
					return 0, err
				}

				val, err := f.getMaxIncomingConnections(meta, podName, IsRepSet(mongodb))
				return val, err
			} else {
				value := int32(-1)
				// shard nodes
				for i := int32(0); i < mongodb.Spec.ShardTopology.Shard.Shards; i++ {
					nodeName := mongodb.ShardNodeName(i)
					podName, err := f.GetReplicaMasterNode(meta, nodeName, &mongodb.Spec.ShardTopology.Shard.Replicas)
					if err != nil {
						return 0, err
					}
					val, err := f.getMaxIncomingConnections(meta, podName, true)
					if err != nil {
						return 0, err
					}
					if value != -1 && val != value {
						return 0, fmt.Errorf("different maxIncomingConnections in different nodes. %v & %v ", val, value)
					}
					value = val
				}

				// config server nodes
				nodeName := mongodb.ConfigSvrNodeName()
				podName, err := f.GetReplicaMasterNode(meta, nodeName, &mongodb.Spec.ShardTopology.ConfigServer.Replicas)
				if err != nil {
					return 0, err
				}
				val, err := f.getMaxIncomingConnections(meta, podName, true)
				if err != nil {
					return 0, err
				}

				if value != -1 && val != value {
					return 0, fmt.Errorf("different maxIncomingConnections in different nodes. %v & %v ", val, value)
				}
				value = val

				// config server nodes
				podName, err = f.GetMongosPodName(meta)
				if err != nil {
					return 0, err
				}
				val, err = f.getMaxIncomingConnections(meta, podName, true)
				if err != nil {
					return 0, err
				}

				if value != -1 && val != value {
					return 0, fmt.Errorf("different maxIncomingConnections in different nodes. %v & %v ", val, value)
				}
				value = val
				return value, nil
			}
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) getClusterAuthModeFromDB(meta metav1.ObjectMeta) (string, error) {
	podName, err := f.GetPrimaryInstance(meta)
	if err != nil {
		log.Errorln("GetPrimaryInstance error:", err)
		return "", err
	}

	client, tunnel, err := f.ConnectAndPing(meta, podName, true)
	if err != nil {
		return "", fmt.Errorf("failed to ConnectAndPing. Reason: %v", err)
	}
	defer tunnel.Close()

	res := make(map[string]interface{})
	err = client.Database("admin").
		RunCommand(context.Background(), bson.D{
			primitive.E{Key: "getParameter", Value: 1},
			primitive.E{Key: "clusterAuthMode", Value: 1},
		}).Decode(&res)
	if err != nil {
		log.Errorln("RunCommand getCmdLineOpts error:", err)
		return "", err
	}

	val, ok := res["clusterAuthMode"]
	if !ok {
		return "", fmt.Errorf("clusterAuthMode not found")
	}

	return val.(string), nil
}

func (f *Framework) getSSLModeFromDB(meta metav1.ObjectMeta) (string, error) {
	podName, err := f.GetPrimaryInstance(meta)
	if err != nil {
		log.Errorln("GetPrimaryInstance error:", err)
		return "", err
	}

	client, tunnel, err := f.ConnectAndPing(meta, podName, true)
	if err != nil {
		return "", fmt.Errorf("failed to ConnectAndPing. Reason: %v", err)
	}
	defer tunnel.Close()

	res := make(map[string]interface{})
	err = client.Database("admin").
		RunCommand(context.Background(), bson.D{
			primitive.E{Key: "getParameter", Value: 1},
			primitive.E{Key: "sslMode", Value: 1},
		}).Decode(&res)
	if err != nil {
		log.Errorln("RunCommand getCmdLineOpts error:", err)
		return "", err
	}

	val, ok := res["sslMode"]
	if !ok {
		return "", fmt.Errorf("sslMode not found")
	}

	return val.(string), nil
}

func (f *Framework) EventuallyUserSSLSettings(meta metav1.ObjectMeta, clusterAuthMode *api.ClusterAuthMode, sslMode *api.SSLMode) GomegaAsyncAssertion {
	return Eventually(
		func() (bool, error) {
			mongodb, err := f.GetMongoDB(meta)
			if err != nil {
				return false, err
			}

			clusterAuth := mongodb.Spec.ClusterAuthMode
			if clusterAuthMode != nil {
				clusterAuth = *clusterAuthMode
			}
			if clusterAuth != "" {
				val, err := f.getClusterAuthModeFromDB(meta)
				if err != nil {
					return false, err
				}
				if val != string(clusterAuth) {
					return false, fmt.Errorf("expected clusterAuthMode %v, but got %v", clusterAuth, val)
				}
			}

			sm := mongodb.Spec.SSLMode
			if sslMode != nil {
				sm = *sslMode
			}
			if sm != "" {
				val, err := f.getSSLModeFromDB(meta)
				if err != nil {
					//log.Infoln(err) //TODO: Fixed some cert-manager problems here. Delete later
					return false, err
				}
				if val != string(sm) {
					return false, fmt.Errorf("expected SSLMode %v, but got %v", sm, val)
				}
			}
			return true, nil
		},
		time.Minute*1,
		time.Second*1,
	)
}

func (f *Framework) getStorageEngine(meta metav1.ObjectMeta, podName string) (string, error) {
	client, tunnel, err := f.ConnectAndPing(meta, podName, true)
	if err != nil {
		return "", fmt.Errorf("failed to ConnectAndPing. Reason: %v", err)
	}
	defer tunnel.Close()

	res := make(map[string]interface{})
	err = client.Database("admin").
		RunCommand(context.Background(), bson.D{
			primitive.E{Key: "serverStatus", Value: 1},
		}).Decode(&res)
	if err != nil {
		log.Errorln("RunCommand serverStatus error:", err)
		return "", err
	}

	if val, ok := res["storageEngine"]; !ok {
		return "", fmt.Errorf("storageEngine Not found")
	} else {
		if se, ok := val.(map[string]interface{}); ok {
			return se["name"].(string), nil
		}
	}

	return "", nil
}
