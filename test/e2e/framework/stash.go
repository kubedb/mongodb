package framework

import (
	"time"

	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/pkg/controller"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	core_util "kmodules.xyz/client-go/core/v1"
	rbac_util "kmodules.xyz/client-go/rbac/v1beta1"
	v1alpha13 "kmodules.xyz/custom-resources/apis/appcatalog/v1alpha1"
	ofst "kmodules.xyz/offshoot-api/api/v1"
	"stash.appscode.dev/stash/apis/stash/v1alpha1"
	stashV1alpha1 "stash.appscode.dev/stash/apis/stash/v1alpha1"
	"stash.appscode.dev/stash/apis/stash/v1beta1"
)

var (
	StashMgBackupTask  = "mongo-backup-task"
	StashMgRestoreTask = "mongo-restore-task"
	StashMgClusterRole = "mongo-backup-restore"
	StashMgSA          = "mongo-backup-restore"
	StashMgRoleBinding = "mongo-backup-restore"
)

func (f *Framework) FoundStashCRDs() bool {
	return controller.FoundStashCRDs(f.apiExtKubeClient)
}

func (i *Invocation) BackupConfiguration(meta metav1.ObjectMeta) *v1beta1.BackupConfiguration {
	return &v1beta1.BackupConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: i.namespace,
		},
		Spec: v1beta1.BackupConfigurationSpec{
			RuntimeSettings: ofst.RuntimeSettings{
				Pod: &ofst.PodRuntimeSettings{
					ServiceAccountName: StashMgSA,
				},
			},
			Task: v1beta1.TaskRef{
				Name: StashMgBackupTask,
			},
			Repository: core.LocalObjectReference{
				Name: meta.Name,
			},
			//Schedule: "*/3 * * * *",
			Target: &v1beta1.BackupTarget{
				Ref: v1beta1.TargetRef{
					APIVersion: v1alpha13.SchemeGroupVersion.String(),
					Kind:       v1alpha13.ResourceKindApp,
					Name:       meta.Name,
				},
			},
			RetentionPolicy: v1alpha1.RetentionPolicy{
				KeepLast: 5,
				Prune:    true,
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

func (f *Framework) EventuallyBackupSessionPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() (phase v1beta1.BackupSessionPhase) {
			bs, err := f.stashClient.StashV1beta1().BackupSessions(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return bs.Status.Phase
		},
	)
}

func (i *Invocation) Repository(meta metav1.ObjectMeta, secretName string) *stashV1alpha1.Repository {
	return &stashV1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: i.namespace,
		},
	}
}

func (f *Framework) CreateRepository(repo *stashV1alpha1.Repository) error {
	_, err := f.stashClient.StashV1alpha1().Repositories(repo.Namespace).Create(repo)

	return err
}

func (f *Framework) DeleteRepository(meta metav1.ObjectMeta) error {
	err := f.stashClient.StashV1alpha1().Repositories(meta.Namespace).Delete(meta.Name, deleteInBackground())
	return err
}

func (i *Invocation) BackupSession(meta metav1.ObjectMeta) *v1beta1.BackupSession {
	return &v1beta1.BackupSession{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: i.namespace,
		},
		Spec: v1beta1.BackupSessionSpec{
			BackupConfiguration: core.LocalObjectReference{
				Name: meta.Name,
			},
		},
	}
}

func (f *Framework) CreateBackupSession(bc *v1beta1.BackupSession) error {
	_, err := f.stashClient.StashV1beta1().BackupSessions(bc.Namespace).Create(bc)
	return err
}

func (f *Framework) DeleteBackupSession(meta metav1.ObjectMeta) error {
	err := f.stashClient.StashV1beta1().BackupSessions(meta.Namespace).Delete(meta.Name, deleteInBackground())
	return err
}

func (i *Invocation) RestoreSession(meta, oldMeta metav1.ObjectMeta) *v1beta1.RestoreSession {
	return &v1beta1.RestoreSession{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: i.namespace,
			Labels: map[string]string{
				"app":                 i.app,
				api.LabelDatabaseKind: api.ResourceKindMongoDB,
			},
		},
		Spec: v1beta1.RestoreSessionSpec{
			RuntimeSettings: ofst.RuntimeSettings{
				Pod: &ofst.PodRuntimeSettings{
					ServiceAccountName: StashMgSA,
				},
			},
			Task: v1beta1.TaskRef{
				Name: StashMgRestoreTask,
			},
			Repository: core.LocalObjectReference{
				Name: oldMeta.Name,
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
					Name:       meta.Name,
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
		time.Minute*7,
		time.Second*7,
	)
}

func (f *Framework) EnsureStashMgRBAC(meta metav1.ObjectMeta) error {
	if err := f.CreateStashMgServiceAccount(meta); err != nil {
		return err
	}
	if err := f.CreateStashMgRoleBinding(meta); err != nil {
		return err
	}
	return nil
}

func (f *Framework) DeleteStashMgRBAC(meta metav1.ObjectMeta) error {
	if err := f.kubeClient.CoreV1().ServiceAccounts(meta.Namespace).Delete(StashMgSA, deleteInForeground()); err != nil {
		return err
	}
	if err := f.kubeClient.RbacV1().RoleBindings(meta.Namespace).Delete(StashMgRoleBinding, deleteInForeground()); err != nil {
		return err
	}
	return nil
}

func (f *Framework) CreateStashMgServiceAccount(meta metav1.ObjectMeta) error {
	// Create new ServiceAccount
	_, _, err := core_util.CreateOrPatchServiceAccount(
		f.kubeClient,
		metav1.ObjectMeta{
			Name:      StashMgSA,
			Namespace: meta.Namespace,
		},
		func(in *core.ServiceAccount) *core.ServiceAccount {
			return in
		},
	)
	return err
}

func (f *Framework) CreateStashMgRoleBinding(meta metav1.ObjectMeta) error {
	// Ensure new RoleBindings
	_, _, err := rbac_util.CreateOrPatchRoleBinding(
		f.kubeClient,
		metav1.ObjectMeta{
			Name:      StashMgRoleBinding,
			Namespace: meta.Namespace,
		},
		func(in *rbac.RoleBinding) *rbac.RoleBinding {
			in.RoleRef = rbac.RoleRef{
				APIGroup: rbac.GroupName,
				Kind:     "ClusterRole",
				Name:     StashMgClusterRole,
			}
			in.Subjects = []rbac.Subject{
				{
					Kind:      rbac.ServiceAccountKind,
					Name:      StashMgSA,
					Namespace: meta.Namespace,
				},
			}
			return in
		},
	)
	return err
}
