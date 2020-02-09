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
	"fmt"
	"os"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/apimachinery/pkg/controller"

	"github.com/appscode/go/log"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1alpha13 "kmodules.xyz/custom-resources/apis/appcatalog/v1alpha1"
	store "kmodules.xyz/objectstore-api/api/v1"
	"stash.appscode.dev/stash/apis/stash/v1alpha1"
	stashV1alpha1 "stash.appscode.dev/stash/apis/stash/v1alpha1"
	"stash.appscode.dev/stash/apis/stash/v1beta1"
	v1beta1_util "stash.appscode.dev/stash/client/clientset/versioned/typed/stash/v1beta1/util"
)

func (f *Framework) FoundStashCRDs() bool {
	return controller.FoundStashCRDs(f.apiExtKubeClient)
}

func (i *Invocation) BackupConfiguration(dbMeta metav1.ObjectMeta, repo *stashV1alpha1.Repository) *v1beta1.BackupConfiguration {
	return &v1beta1.BackupConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbMeta.Name + "-stash",
			Namespace: i.namespace,
		},
		Spec: v1beta1.BackupConfigurationSpec{
			Repository: core.LocalObjectReference{
				Name: repo.Name,
			},
			RetentionPolicy: v1alpha1.RetentionPolicy{
				Name:     "keep-last-5",
				KeepLast: 5,
				Prune:    true,
			},
			Schedule: "*/2 * * * *",
			BackupConfigurationTemplateSpec: v1beta1.BackupConfigurationTemplateSpec{
				Task: v1beta1.TaskRef{
					Name: i.getStashMGBackupTaskName(),
				},
				Target: &v1beta1.BackupTarget{
					Ref: v1beta1.TargetRef{
						APIVersion: v1alpha13.SchemeGroupVersion.String(),
						Kind:       v1alpha13.ResourceKindApp,
						Name:       dbMeta.Name,
					},
				},
			},
		},
	}
}

func (f *Framework) CreateBackupConfiguration(backupCfg *v1beta1.BackupConfiguration) error {
	_, err := f.stashClient.StashV1beta1().BackupConfigurations(backupCfg.Namespace).Create(backupCfg)
	return err
}

func (f *Framework) DeleteBackupConfiguration(meta metav1.ObjectMeta) error {
	return f.stashClient.StashV1beta1().BackupConfigurations(meta.Namespace).Delete(meta.Name, &metav1.DeleteOptions{})
}

func (f *Framework) PauseBackupConfiguration(meta metav1.ObjectMeta) error {
	_, err := v1beta1_util.TryUpdateBackupConfiguration(f.stashClient.StashV1beta1(), meta, func(in *v1beta1.BackupConfiguration) *v1beta1.BackupConfiguration {
		in.Spec.Paused = true
		return in
	})
	return err
}

func (i *Invocation) Repository(dbMeta metav1.ObjectMeta, secretName string) *stashV1alpha1.Repository {
	return &stashV1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbMeta.Name + "-stash",
			Namespace: i.namespace,
		},
		Spec: stashV1alpha1.RepositorySpec{
			Backend: i.backendSpec(dbMeta, secretName),
			WipeOut: true,
		},
	}
}

func (i *Invocation) backendSpec(dbMeta metav1.ObjectMeta, secretName string) store.Backend {
	switch StorageProvider {
	case StorageProviderGCS:
		return i.gcsBackend(dbMeta, secretName)
	case StorageProviderS3:
		return i.s3Backend(dbMeta, secretName)
	case StorageProviderMinio:
		return i.minioBackend(dbMeta, secretName)
	case StorageProviderAzure:
		return i.azureBackend(dbMeta, secretName)
	case StorageProviderSwift:
		return i.swiftBackend(dbMeta, secretName)
	}
	return store.Backend{}
}

func (i *Invocation) gcsBackend(dbMeta metav1.ObjectMeta, secretName string) store.Backend {
	return store.Backend{
		GCS: &store.GCSSpec{
			Bucket: os.Getenv("GCS_BUCKET_NAME"),
			Prefix: fmt.Sprintf("stash/%v/%v", dbMeta.Namespace, dbMeta.Name),
		},
		StorageSecretName: secretName,
	}
}

func (i *Invocation) s3Backend(dbMeta metav1.ObjectMeta, secretName string) store.Backend {
	return store.Backend{
		S3: &store.S3Spec{
			Bucket: os.Getenv("S3_BUCKET_NAME"),
			Prefix: fmt.Sprintf("stash/%v/%v", dbMeta.Namespace, dbMeta.Name),
		},
		StorageSecretName: secretName,
	}
}
func (i *Invocation) minioBackend(dbMeta metav1.ObjectMeta, secretName string) store.Backend {
	return store.Backend{
		S3: &store.S3Spec{
			Endpoint: i.MinioServiceAddres(),
			Bucket:   i.app,
			Prefix:   fmt.Sprintf("stash/%v/%v", dbMeta.Namespace, dbMeta.Name),
		},
		StorageSecretName: secretName,
	}
}
func (i *Invocation) azureBackend(dbMeta metav1.ObjectMeta, secretName string) store.Backend {
	return store.Backend{
		Azure: &store.AzureSpec{
			Container: os.Getenv("AZURE_CONTAINER_NAME"),
			Prefix:    fmt.Sprintf("stash/%v/%v", dbMeta.Namespace, dbMeta.Name),
		},
		StorageSecretName: secretName,
	}
}
func (i *Invocation) swiftBackend(dbMeta metav1.ObjectMeta, secretName string) store.Backend {
	return store.Backend{
		Swift: &store.SwiftSpec{
			Container: os.Getenv("SWIFT_CONTAINER_NAME"),
			Prefix:    fmt.Sprintf("stash/%v/%v", dbMeta.Namespace, dbMeta.Name),
		},
		StorageSecretName: secretName,
	}
}

func (f *Framework) CreateRepository(repo *stashV1alpha1.Repository) error {
	_, err := f.stashClient.StashV1alpha1().Repositories(repo.Namespace).Create(repo)

	return err
}

func (f *Framework) DeleteRepository(meta metav1.ObjectMeta) error {
	err := f.stashClient.StashV1alpha1().Repositories(meta.Namespace).Delete(meta.Name, deleteInForeground())
	return err
}

func (f *Framework) EventuallySnapshotInRepository(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() int64 {
			repository, err := f.stashClient.StashV1alpha1().Repositories(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			log.Infoln(fmt.Sprintf("Found %v snapshot(s)", repository.Status.SnapshotCount))
			return repository.Status.SnapshotCount
		},
		time.Minute*13,
		time.Second*5,
	)
}

func (i *Invocation) RestoreSession(dbMeta metav1.ObjectMeta, repo *stashV1alpha1.Repository) *v1beta1.RestoreSession {
	return &v1beta1.RestoreSession{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dbMeta.Name + "-stash",
			Namespace: i.namespace,
			Labels: map[string]string{
				"app":                 i.app,
				api.LabelDatabaseKind: api.ResourceKindMongoDB,
			},
		},
		Spec: v1beta1.RestoreSessionSpec{
			Task: v1beta1.TaskRef{
				Name: i.getStashMGRestoreTaskName(),
			},
			Repository: core.LocalObjectReference{
				Name: repo.Name,
			},
			Rules: []v1beta1.Rule{
				{
					Snapshots: []string{"latest"},
				},
			},
			Target: &v1beta1.RestoreTarget{
				Ref: v1beta1.TargetRef{
					APIVersion: v1alpha13.SchemeGroupVersion.String(),
					Kind:       v1alpha13.ResourceKindApp,
					Name:       dbMeta.Name,
				},
			},
		},
	}
}

func (f *Framework) CreateRestoreSession(restoreSession *v1beta1.RestoreSession) error {
	_, err := f.stashClient.StashV1beta1().RestoreSessions(restoreSession.Namespace).Create(restoreSession)
	return err
}

func (f Framework) DeleteRestoreSession(meta metav1.ObjectMeta) error {
	err := f.stashClient.StashV1beta1().RestoreSessions(meta.Namespace).Delete(meta.Name, &metav1.DeleteOptions{})
	return err
}

func (f *Framework) EventuallyRestoreSessionPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(func() v1beta1.RestoreSessionPhase {
		restoreSession, err := f.stashClient.StashV1beta1().RestoreSessions(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		return restoreSession.Status.Phase
	},
		time.Minute*13,
		time.Second*5,
	)
}

func (f *Framework) getStashMGBackupTaskName() string {
	esVersion, err := f.dbClient.CatalogV1alpha1().MongoDBVersions().Get(DBCatalogName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	return "mongodb-backup-" + esVersion.Spec.Version
}

func (f *Framework) getStashMGRestoreTaskName() string {
	mongoVersion, err := f.dbClient.CatalogV1alpha1().MongoDBVersions().Get(DBCatalogName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())
	if mongoVersion.Spec.Version == "4.0.3" {
		// mongorestore may not work for Replicaset and Sharding for 4.0.3. Use `4.0.11` image for restore purpose. issue link: http://mongodb.2344371.n4.nabble.com/mongorestore-oplogReplay-looping-forever-td25243.html
		mongoVersion.Spec.Version = "4.0.11"
	}
	return "mongodb-restore-" + mongoVersion.Spec.Version
}
