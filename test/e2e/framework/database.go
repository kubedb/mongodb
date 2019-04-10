package framework

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson"
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

func (f *Framework) GetMongoDBClient(meta metav1.ObjectMeta, tunnel *portforward.Tunnel, isReplSet bool) (*options.ClientOptions, error) {
	mongodb, err := f.GetMongoDB(meta)
	if err != nil {
		return nil, err
	}

	user := "root"
	pass, err := f.GetMongoDBRootPassword(mongodb)

	clientOpts := options.Client().ApplyURI(fmt.Sprintf("mongodb://%s:%s@127.0.0.1:%v", user, pass, tunnel.Local))
	if isReplSet {
		clientOpts.SetDirect(true)
	}
	return clientOpts, nil
}

func (f *Framework) GetMongosPodName(meta metav1.ObjectMeta) (string, error) {
	pods, err := f.kubeClient.CoreV1().Pods(meta.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return "", err
	}
	for _, pod := range pods.Items {
		if strings.HasPrefix(pod.Name, fmt.Sprintf("%v-mongos", meta.Name)) {
			return pod.Name, nil
		}
	}
	return "", fmt.Errorf("no pod found for memcache: %s", meta.Name)
}

func (f *Framework) GetPrimaryInstance(meta metav1.ObjectMeta, isReplSet bool) (string, error) {
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

	fn := func(clientPodName string) (bool, error) {
		tunnel, err := f.ForwardPort(meta, clientPodName)
		if err != nil {
			return false, err
		}
		defer tunnel.Close()

		clientOpts, err := f.GetMongoDBClient(meta, tunnel, isReplSet)
		if err != nil {
			return false, err
		}

		client, err := mongo.Connect(context.Background(), clientOpts)
		if err != nil {
			return false, err
		}

		err = client.Ping(context.TODO(), nil)
		if err != nil {
			return false, err
		}

		res := make(map[string]interface{})
		if err := client.Database("admin").RunCommand(context.Background(), bson.D{{"isMaster", "1"}}).Decode(&res); err != nil {
			return false, err
		}

		if val, ok := res["ismaster"]; ok && val == true {
			return true, nil
		}
		return false, fmt.Errorf("%v not master node", clientPodName)
	}

	// For MongoDB ReplicaSet, Find out the primary instance.
	// Extract information `IsMaster: true` from the component's status.
	for i := *mongodb.Spec.Replicas - 1; i >= 0; i-- {
		clientPodName := fmt.Sprintf("%v-%d", mongodb.Name, i)
		var isMaster bool
		isMaster, err = fn(clientPodName)
		if err == nil && isMaster {
			return clientPodName, nil
		}
	}
	return "", err
}

func (f *Framework) EventuallyInsertDocument(meta metav1.ObjectMeta, dbName string, isRepset bool, collectionCount int) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			podName, err := f.GetPrimaryInstance(meta, isRepset)
			if err != nil {
				fmt.Println("GetPrimaryInstance error:", err)
				return false
			}

			tunnel, err := f.ForwardPort(meta, podName)
			if err != nil {
				fmt.Println("Failed to forward port. Reason: ", err)
				return false
			}
			defer tunnel.Close()

			clientOpts, err := f.GetMongoDBClient(meta, tunnel, isRepset)
			if err != nil {
				fmt.Println("GetMongoDB Client error:", err)
			}

			client, err := mongo.Connect(context.Background(), clientOpts)
			if err != nil {
				fmt.Println("GetMongoDB Client error:", err)
				return false
			}

			defer func() {
				if err := client.Disconnect(context.Background()); err != nil {
					fmt.Println("GetMongoDB Client error:", err)
				}
			}()

			err = client.Ping(context.TODO(), nil)
			if err != nil {
				fmt.Println("Ping error:", err)
				return false
			}

			person := &KubedbTable{
				FirstName: "kubernetes",
				LastName:  "database",
			}

			if _, err := client.Database(dbName).Collection("people").InsertOne(context.Background(), person); err != nil {
				fmt.Println("creation error:", err)
				return false
			}

			// above one is 0th element
			for i := 1; i < collectionCount; i++ {
				person := &KubedbTable{
					FirstName: fmt.Sprintf("kubernetes-%03d", i),
					LastName:  fmt.Sprintf("database-%03d", i),
				}

				if _, err := client.Database(dbName).Collection(fmt.Sprintf("people-%03d", i)).InsertOne(context.Background(), person); err != nil {
					fmt.Println("creation error:", err)
					return false
				}
			}

			return true
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) EventuallyDocumentExists(meta metav1.ObjectMeta, dbName string, isReplSet bool, collectionCount int) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			podName, err := f.GetPrimaryInstance(meta, isReplSet)
			if err != nil {
				fmt.Println("GetPrimaryInstance error:", err)
				return false
			}

			tunnel, err := f.ForwardPort(meta, podName)
			if err != nil {
				fmt.Println("Failed to forward port. Reason: ", err)
				return false
			}
			defer tunnel.Close()

			clientOpts, err := f.GetMongoDBClient(meta, tunnel, isReplSet)
			if err != nil {
				fmt.Println("GetMongoDB Client error:", err)
			}

			client, err := mongo.Connect(context.Background(), clientOpts)
			if err != nil {
				fmt.Println("GetMongoDB Client error:", err)
				return false
			}

			defer func() {
				if err := client.Disconnect(context.Background()); err != nil {
					fmt.Println("GetMongoDB Client error:", err)
				}
			}()

			err = client.Ping(context.TODO(), nil)
			if err != nil {
				fmt.Println("Ping error:", err)
				return false
			}
			expected := &KubedbTable{
				FirstName: "kubernetes",
				LastName:  "database",
			}
			person := &KubedbTable{}

			if er := client.Database(dbName).Collection("people").FindOne(context.Background(), bson.M{"firstname": expected.FirstName}).Decode(&person); er != nil || person == nil || person.LastName != expected.LastName {
				fmt.Println("checking error:", er)
				return false
			}

			// above one is 0th element
			for i := 1; i < collectionCount; i++ {
				expected := &KubedbTable{
					FirstName: fmt.Sprintf("kubernetes-%03d", i),
					LastName:  fmt.Sprintf("database-%03d", i),
				}
				person := &KubedbTable{}

				if er := client.Database(dbName).Collection(fmt.Sprintf("people-%03d", i)).FindOne(context.Background(), bson.M{"firstname": expected.FirstName}).Decode(&person); er != nil || person == nil || person.LastName != expected.LastName {
					fmt.Println("checking error:", er)
					return false
				}
			}
			return true
		},
		time.Minute*5,
		time.Second*5,
	)
}
