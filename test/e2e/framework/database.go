package framework

import (
	"fmt"
	"time"

	"github.com/appscode/kutil/tools/portforward"
	"github.com/globalsign/mgo/bson"
	"github.com/go-bongo/bongo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type KubedbTable struct {
	bongo.DocumentBase `bson:",inline"`
	FirstName          string
	LastName           string
}

func (f *Framework) GetMongoDBClient(meta metav1.ObjectMeta) (*bongo.Connection, error) {
	mongodb, err := f.GetMongoDB(meta)
	if err != nil {
		return nil, err
	}
	clientPodName := fmt.Sprintf("%v-0", mongodb.Name)
	tunnel := portforward.NewTunnel(
		f.kubeClient.CoreV1().RESTClient(),
		f.restConfig,
		mongodb.Namespace,
		clientPodName,
		27017,
	)

	if err := tunnel.ForwardPort(); err != nil {
		return nil, err
	}
	user := "root"
	pass, err := f.GetMongoDBRootPassword(mongodb)

	config := &bongo.Config{
		ConnectionString: fmt.Sprintf("mongodb://%s:%s@127.0.0.1:%v", user, pass, tunnel.Local),
		Database:         "kubedb",
	}

	return bongo.Connect(config)

}

func (f *Framework) EventuallyInsertDocument(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			en, err := f.GetMongoDBClient(meta)
			if err != nil {
				return false
			}
			defer en.Session.Close()

			if err := en.Session.Ping(); err != nil {
				return false
			}

			person := &KubedbTable{
				FirstName: "kubernetes",
				LastName:  "database",
			}

			if err := en.Collection("people").Save(person); err != nil {
				fmt.Println("creation error", err)
				return false
			}
			return true
		},
		time.Minute*15,
		time.Second*10,
	)
}

func (f *Framework) EventuallyDocumentExists(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			en, err := f.GetMongoDBClient(meta)
			if err != nil {
				return false
			}
			defer en.Session.Close()

			if err := en.Session.Ping(); err != nil {
				return false
			}
			person := &KubedbTable{}

			if err := en.Collection("people").FindOne(bson.M{"firstname": "kubernetes"}, person); err == nil {
				return true
			}
			return false
		},
		time.Minute*15,
		time.Second*10,
	)
}
