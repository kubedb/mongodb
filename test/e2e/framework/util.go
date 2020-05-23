/*
Copyright The KubeDB Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package framework

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

	"github.com/appscode/go/log"
	"github.com/aws/aws-sdk-go/aws"
	shell "github.com/codeskyblue/go-sh"
	promClient "github.com/prometheus/client_model/go"
	"github.com/prometheus/prom2json"
	v1 "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	kutil "kmodules.xyz/client-go"
	meta_util "kmodules.xyz/client-go/meta"
	"kmodules.xyz/client-go/tools/portforward"
	kmon "kmodules.xyz/monitoring-agent-api/api/v1"
)

const (
	updateRetryInterval  = 10 * 1000 * 1000 * time.Nanosecond
	maxAttempts          = 5
	mongodbUpMetric      = "mongodb_up"
	metricsMatchedCount  = 2
	mongodbVersionMetric = "mongodb_version_info"
)

func (f *Framework) DeleteCASecret(clientCASecret *v1.Secret) {
	err := f.CheckSecret(clientCASecret)
	if err != nil {
		return
	}
	if err := f.DeleteSecret(clientCASecret.ObjectMeta); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of CA secret. Error: %v", err)
	}
}

func (f *Framework) DeleteGarbageCASecrets(secretList []*v1.Secret) {
	if len(secretList) == 0 {
		return
	}
	for _, secret := range secretList {
		f.DeleteCASecret(secret)
	}
}

func (f *Framework) CleanWorkloadLeftOvers() {
	// delete statefulset
	if err := f.kubeClient.AppsV1().StatefulSets(f.namespace).DeleteCollection(context.TODO(), meta_util.DeleteInForeground(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			api.LabelDatabaseKind: api.ResourceKindMongoDB,
		}).String(),
	}); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of Statefulset. Error: %v", err)
	}

	// delete pvc
	if err := f.kubeClient.CoreV1().PersistentVolumeClaims(f.namespace).DeleteCollection(context.TODO(), meta_util.DeleteInForeground(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			api.LabelDatabaseKind: api.ResourceKindMongoDB,
		}).String(),
	}); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of PVC. Error: %v", err)
	}
}

func (f *Framework) AddMonitor(obj *api.MongoDB) {
	obj.Spec.Monitor = &kmon.AgentSpec{
		Prometheus: &kmon.PrometheusSpec{
			Exporter: &kmon.PrometheusExporterSpec{
				Port:            api.PrometheusExporterPortNumber,
				Resources:       v1.ResourceRequirements{},
				SecurityContext: nil,
			},
		},
		Agent: kmon.AgentPrometheus,
	}
}

func (f *Framework) VerifyShardExporters(meta metav1.ObjectMeta) error {
	mongoDB, err := f.dbClient.KubedbV1alpha1().MongoDBs(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
	if err != nil {
		log.Infoln(err)
		return err
	}

	newMeta := metav1.ObjectMeta{
		Name:      "",
		Namespace: meta.Namespace,
	}
	// for config server
	newMeta.Name = mongoDB.ConfigSvrNodeName()
	err = f.VerifyExporter(newMeta)
	if err != nil {
		log.Infoln(err)
		return err
	}
	// for shards
	newMeta.Name = mongoDB.ShardNodeName(int32(0))
	err = f.VerifyExporter(newMeta)
	if err != nil {
		log.Infoln(err)
		return err
	}
	// for mongos
	newMeta.Name = mongoDB.MongosNodeName()
	err = f.VerifyExporter(newMeta)
	if err != nil {
		log.Infoln(err)
		return err
	}

	return nil
}

//VerifyExporter uses metrics from given URL
//and check against known key and value
//to verify the connection is functioning as intended
func (f *Framework) VerifyExporter(meta metav1.ObjectMeta) error {
	tunnel, err := f.ForwardToPort(meta, fmt.Sprintf("%v-0", meta.Name), aws.Int(api.PrometheusExporterPortNumber))
	if err != nil {
		log.Infoln(err)
		return err
	}
	defer tunnel.Close()
	return wait.PollImmediate(time.Second, kutil.ReadinessTimeout, func() (bool, error) {
		metricsURL := fmt.Sprintf("http://127.0.0.1:%d/metrics", tunnel.Local)
		mfChan := make(chan *promClient.MetricFamily, 1024)
		transport := makeTransport()

		err := prom2json.FetchMetricFamilies(metricsURL, mfChan, transport)
		if err != nil {
			log.Infoln(err)
			return false, nil
		}

		var count = 0
		for mf := range mfChan {
			if mf.Metric != nil && mf.Metric[0].Gauge != nil && mf.Metric[0].Gauge.Value != nil {
				if *mf.Name == mongodbVersionMetric && strings.Contains(DBCatalogName, *mf.Metric[0].Label[0].Value) {
					count++
				} else if *mf.Name == mongodbUpMetric && int(*mf.Metric[0].Gauge.Value) > 0 {
					count++
				}
			}
		}

		if count != metricsMatchedCount {
			return false, nil
		}
		log.Infoln("Found ", count, " metrics out of ", metricsMatchedCount)
		return true, nil
	})
}
func makeTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
}

func (f *Framework) ForwardToPort(meta metav1.ObjectMeta, clientPodName string, port *int) (*portforward.Tunnel, error) {
	var defaultPort = api.PrometheusExporterPortNumber
	if port != nil {
		defaultPort = *port
	}

	tunnel := portforward.NewTunnel(
		f.kubeClient.CoreV1().RESTClient(),
		f.restConfig,
		meta.Namespace,
		clientPodName,
		defaultPort,
	)
	if err := tunnel.ForwardPort(); err != nil {
		return nil, err
	}

	return tunnel, nil
}

func (f *Framework) PrintDebugHelpers() {
	sh := shell.NewSession()
	fmt.Println("\n======================================[ Describe Nodes ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "get", "nodes").Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe Job ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "job", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe Pod ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "po", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe Mongo ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "mg", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe BackupSession ]==========================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "backupsession", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe RestoreSession ]==========================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "restoresession", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe Nodes ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "nodes").Run(); err != nil {
		fmt.Println(err)
	}
}
