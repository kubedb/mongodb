package framework

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/appscode/go/log"
	"github.com/kubedb/kubedb-server/pkg/admission/plugin/dormant-database"
	"github.com/kubedb/kubedb-server/pkg/admission/plugin/mongodb"
	"github.com/kubedb/kubedb-server/pkg/admission/plugin/snapshot"
	"github.com/kubedb/kubedb-server/pkg/cmds/server"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kApi "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1beta1"
)

func (f *Framework) EventuallyApiServiceReady() GomegaAsyncAssertion {
	return Eventually(
		func() error {
			crd, err := f.kaClient.ApiregistrationV1beta1().APIServices().Get("v1alpha1.admission.kubedb.com", metav1.GetOptions{})
			if err != nil {
				return err
			}
			for _, cond := range crd.Status.Conditions {
				if cond.Type == kApi.Available && cond.Status == kApi.ConditionTrue {
					time.Sleep(time.Second * 5) // let the resource become available
					log.Info("APIService status is true")
					return nil
				}
			}
			log.Error("APIService not ready yet")
			return fmt.Errorf("APIService not ready yet")
		},
		time.Minute*2,
		time.Second*5,
	)
}

func (f *Framework) RunAdmissionServer(kubeconfigPath string, stopCh <-chan struct{}) {
	serverOpt := server.NewAdmissionServerOptions(os.Stdout, os.Stderr,
		&mongodb.MongoDBValidator{}, &mongodb.MongoDBMutator{},
		&snapshot.SnapshotValidator{},
		&dormant_database.DormantDatabaseValidator{})

	serverOpt.RecommendedOptions.CoreAPI.CoreAPIKubeconfigPath = kubeconfigPath
	serverOpt.RecommendedOptions.SecureServing.BindPort = 8443
	serverOpt.RecommendedOptions.SecureServing.BindAddress = net.ParseIP("127.0.0.1")
	serverOpt.RecommendedOptions.Authorization.RemoteKubeConfigFile = kubeconfigPath
	serverOpt.RecommendedOptions.Authentication.RemoteKubeConfigFile = kubeconfigPath
	serverOpt.RecommendedOptions.Authentication.SkipInClusterLookup = true
	serverOpt.RunAdmissionServer(stopCh)
}

func (f *Framework) CleanAdmissionController() {

	// Delete Service
	if err := f.kubeClient.CoreV1().Services("kube-system").Delete("kubedb-operator", deleteInBackground()); err != nil {
		fmt.Errorf("error in deletion of Service. Error: %v", err)
	}

	// Delete EndPoints
	if err := f.kubeClient.CoreV1().Endpoints("kube-system").DeleteCollection(deleteInBackground(), metav1.ListOptions{
		LabelSelector: "app=kubedb",
	}); err != nil {
		fmt.Errorf("error in deletion of Endpoints. Error: %v", err)
	}

	// delete validating Webhook
	if err := f.kubeClient.AdmissionregistrationV1beta1().ValidatingWebhookConfigurations().DeleteCollection(deleteInBackground(), metav1.ListOptions{
		LabelSelector: "app=kubedb",
	}); err != nil {
		fmt.Errorf("error in deletion of Validating Webhhok. Error: %v", err)
	}

	// delete mutating Webhook
	if err := f.kubeClient.AdmissionregistrationV1beta1().MutatingWebhookConfigurations().DeleteCollection(deleteInBackground(), metav1.ListOptions{
		LabelSelector: "app=kubedb",
	}); err != nil {
		fmt.Errorf("error in deletion of MutatingWebhook. Error: %v", err)
	}

	// Delete APIService
	if err := f.kaClient.ApiregistrationV1beta1().APIServices().DeleteCollection(deleteInBackground(), metav1.ListOptions{
		LabelSelector: "app=kubedb",
	}); err != nil {
		fmt.Errorf("error in deletion of APIService. Error: %v", err)
	}
}
