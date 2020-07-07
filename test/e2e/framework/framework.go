/*
Copyright AppsCode Inc. and Contributors

Licensed under the PolyForm Noncommercial License 1.0.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://github.com/appscode/licenses/raw/1.0.0/PolyForm-Noncommercial-1.0.0.md

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package framework

import (
	"context"
	"path/filepath"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	cs "kubedb.dev/apimachinery/client/clientset/versioned"

	"github.com/appscode/go/crypto/rand"
	cm "github.com/jetstack/cert-manager/pkg/client/clientset/versioned"
	"github.com/spf13/afero"
	"gomodules.xyz/cert/certstore"
	crd_cs "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	ka "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	core_util "kmodules.xyz/client-go/core/v1"
	appcat_cs "kmodules.xyz/custom-resources/client/clientset/versioned/typed/appcatalog/v1alpha1"
	scs "stash.appscode.dev/apimachinery/client/clientset/versioned"
)

var (
	DockerRegistry  = "kubedbci"
	DBCatalogName   = "4.1.13-v1"
	StorageProvider string
)

type Framework struct {
	restConfig        *rest.Config
	kubeClient        kubernetes.Interface
	apiExtKubeClient  crd_cs.ApiextensionsV1beta1Interface
	dbClient          cs.Interface
	kaClient          ka.Interface
	appCatalogClient  appcat_cs.AppcatalogV1alpha1Interface
	stashClient       scs.Interface
	topology          *core_util.Topology
	namespace         string
	name              string
	StorageClass      string
	CertStore         *certstore.CertStore
	certManagerClient cm.Interface
}

func New(
	restConfig *rest.Config,
	kubeClient kubernetes.Interface,
	apiExtKubeClient crd_cs.ApiextensionsV1beta1Interface,
	dbClient cs.Interface,
	kaClient ka.Interface,
	appCatalogClient appcat_cs.AppcatalogV1alpha1Interface,
	stashClient scs.Interface,
	storageClass string,
	certManagerClient cm.Interface,
) (*Framework, error) {
	topology, err := core_util.DetectTopology(context.TODO(), kubeClient)
	if err != nil {
		return nil, err
	}
	store, err := certstore.NewCertStore(afero.NewMemMapFs(), filepath.Join("", "pki"))
	if err != nil {
		return nil, err
	}

	err = store.InitCA()
	if err != nil {
		return nil, err
	}

	return &Framework{
		restConfig:        restConfig,
		kubeClient:        kubeClient,
		apiExtKubeClient:  apiExtKubeClient,
		dbClient:          dbClient,
		kaClient:          kaClient,
		appCatalogClient:  appCatalogClient,
		stashClient:       stashClient,
		name:              "mongodb-operator",
		namespace:         rand.WithUniqSuffix(api.ResourceSingularMongoDB),
		StorageClass:      storageClass,
		topology:          topology,
		CertStore:         store,
		certManagerClient: certManagerClient,
	}, nil
}

func (f *Framework) Invoke() *Invocation {
	return &Invocation{
		Framework: f,
		app:       rand.WithUniqSuffix("mongodb-e2e"),
	}
}

func (i *Invocation) DBClient() cs.Interface {
	return i.dbClient
}

type Invocation struct {
	*Framework
	app string
}
