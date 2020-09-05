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
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/mongodb/pkg/controller"

	. "github.com/onsi/gomega"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	meta_util "kmodules.xyz/client-go/meta"
	appcat "kmodules.xyz/custom-resources/apis/appcatalog/v1alpha1"
	appcat_util "kmodules.xyz/custom-resources/client/clientset/versioned/typed/appcatalog/v1alpha1/util"
)

func (f *Framework) EventuallyAppBinding(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() bool {
			_, err := f.appCatalogClient.AppBindings(meta.Namespace).Get(context.TODO(), meta.Name, metav1.GetOptions{})
			if err != nil {
				if kerr.IsNotFound(err) {
					return false
				}
				Expect(err).NotTo(HaveOccurred())
			}
			return true
		},
		time.Minute*5,
		time.Second*5,
	)
}

func (f *Framework) CheckAppBindingSpec(meta metav1.ObjectMeta) error {
	mongodb, err := f.GetMongoDB(meta)
	Expect(err).NotTo(HaveOccurred())

	appBinding, err := f.appCatalogClient.AppBindings(mongodb.Namespace).Get(context.TODO(), mongodb.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	if appBinding.Spec.ClientConfig.Service == nil ||
		appBinding.Spec.ClientConfig.Service.Name != mongodb.ServiceName() ||
		appBinding.Spec.ClientConfig.Service.Port != controller.MongoDBPort {
		return fmt.Errorf("appbinding %v/%v contains invalid data", appBinding.Namespace, appBinding.Name)
	}
	if appBinding.Spec.Secret == nil ||
		(appBinding.Spec.ClientConfig.CABundle == nil && appBinding.Spec.Secret.Name != mongodb.Spec.DatabaseSecret.SecretName) ||
		(appBinding.Spec.ClientConfig.CABundle != nil && appBinding.Spec.Secret.Name != mongodb.MustCertSecretName(api.MongoDBClientCert, "")) {
		return fmt.Errorf("appbinding %v/%v contains invalid secret", appBinding.Namespace, appBinding.Name)
	}
	return nil
}

// EnsureCustomAppBinding creates custom Appbinding for mongodb. In this custom appbinding,
// all fields are similar to actual appbinding object, except Spec.Parameters.
func (f *Framework) EnsureCustomAppBinding(db *api.MongoDB, customAppBindingName string) error {
	appmeta := db.AppBindingMeta()
	// get app binding
	appBinding, err := f.appCatalogClient.AppBindings(db.Namespace).Get(context.TODO(), appmeta.Name(), metav1.GetOptions{})
	if err != nil {
		return err
	}

	meta := metav1.ObjectMeta{
		Name:      customAppBindingName,
		Namespace: db.Namespace,
	}

	if _, _, err := appcat_util.CreateOrPatchAppBinding(
		context.TODO(),
		f.appCatalogClient,
		meta,
		func(in *appcat.AppBinding) *appcat.AppBinding {
			in.Labels = appBinding.Labels
			in.Annotations = appBinding.Annotations

			in.Spec.Type = appBinding.Spec.Type
			in.Spec.ClientConfig.Service = appBinding.Spec.ClientConfig.Service
			in.Spec.ClientConfig.InsecureSkipTLSVerify = appBinding.Spec.ClientConfig.InsecureSkipTLSVerify
			in.Spec.Secret = appBinding.Spec.Secret
			// ignore appBinding.Spec.Parameters

			return in
		},
		metav1.PatchOptions{},
	); err != nil {
		return err
	}

	return nil
}

// DeleteAppBinding deletes the custom appBinding that is created in test
func (f *Framework) DeleteAppBinding(meta metav1.ObjectMeta) error {
	return f.appCatalogClient.AppBindings(meta.Namespace).Delete(context.TODO(), meta.Name, meta_util.DeleteInForeground())
}
