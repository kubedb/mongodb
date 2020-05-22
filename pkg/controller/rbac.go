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
package controller

import (
	"context"
	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	policy_v1beta1 "k8s.io/api/policy/v1beta1"
	rbac "k8s.io/api/rbac/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	core_util "kmodules.xyz/client-go/core/v1"
	rbac_util "kmodules.xyz/client-go/rbac/v1"
	v1 "kmodules.xyz/offshoot-api/api/v1"
)

func (c *Controller) createServiceAccount(db *api.MongoDB, saName string) error {
	owner := metav1.NewControllerRef(db, api.SchemeGroupVersion.WithKind(api.ResourceKindMongoDB))
	// Create new ServiceAccount
	_, _, err := core_util.CreateOrPatchServiceAccount(
		c.Client,
		metav1.ObjectMeta{
			Name:      saName,
			Namespace: db.Namespace,
		},
		func(in *core.ServiceAccount) *core.ServiceAccount {
			core_util.EnsureOwnerReference(&in.ObjectMeta, owner)
			in.Labels = db.OffshootLabels()
			return in
		},
	)
	return err
}

func (c *Controller) ensureRole(db *api.MongoDB, name string, pspName string) error {
	owner := metav1.NewControllerRef(db, api.SchemeGroupVersion.WithKind(api.ResourceKindMongoDB))

	// Create new Role for ElasticSearch and it's Snapshot
	_, _, err := rbac_util.CreateOrPatchRole(
		c.Client,
		metav1.ObjectMeta{
			Name:      name,
			Namespace: db.Namespace,
		},
		func(in *rbac.Role) *rbac.Role {
			core_util.EnsureOwnerReference(&in.ObjectMeta, owner)
			in.Labels = db.OffshootLabels()
			in.Rules = []rbac.PolicyRule{}
			if pspName != "" {
				pspRule := rbac.PolicyRule{
					APIGroups:     []string{policy_v1beta1.GroupName},
					Resources:     []string{"podsecuritypolicies"},
					Verbs:         []string{"use"},
					ResourceNames: []string{pspName},
				}
				in.Rules = append(in.Rules, pspRule)
			}
			// keep this rule even if the psp needs to be removed.
			// rbac for secret is used by init-containers to read cert-secret
			secretRule := rbac.PolicyRule{
				APIGroups: []string{""},
				Resources: []string{"secrets"},
				Verbs:     []string{"get"},
			}
			in.Rules = append(in.Rules, secretRule)
			return in
		},
	)
	return err
}

func (c *Controller) createRoleBinding(db *api.MongoDB, roleName string, saName string) error {
	owner := metav1.NewControllerRef(db, api.SchemeGroupVersion.WithKind(api.ResourceKindMongoDB))
	// Ensure new RoleBindings for ElasticSearch and it's Snapshot
	_, _, err := rbac_util.CreateOrPatchRoleBinding(
		c.Client,
		metav1.ObjectMeta{
			Name:      roleName,
			Namespace: db.Namespace,
		},
		func(in *rbac.RoleBinding) *rbac.RoleBinding {
			core_util.EnsureOwnerReference(&in.ObjectMeta, owner)
			in.Labels = db.OffshootLabels()
			in.RoleRef = rbac.RoleRef{
				APIGroup: rbac.GroupName,
				Kind:     "Role",
				Name:     roleName,
			}
			in.Subjects = []rbac.Subject{
				{
					Kind:      rbac.ServiceAccountKind,
					Name:      saName,
					Namespace: db.Namespace,
				},
			}
			return in
		},
	)
	return err
}

func (c *Controller) getPolicyNames(db *api.MongoDB) (string, error) {
	dbVersion, err := c.ExtClient.CatalogV1alpha1().MongoDBVersions().Get(string(db.Spec.Version), metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	dbPolicyName := dbVersion.Spec.PodSecurityPolicies.DatabasePolicyName

	return dbPolicyName, nil
}

func (c *Controller) ensureDatabaseRBAC(mongodb *api.MongoDB) error {
	var createDatabaseRBAC = func(podTemplate *v1.PodTemplateSpec) error {
		if podTemplate == nil {
			return errors.New("Pod Template can not be empty.")
		}

		saName := podTemplate.Spec.ServiceAccountName
		if saName == "" {
			saName = mongodb.OffshootName() // in case mutator was disabled
			podTemplate.Spec.ServiceAccountName = saName
		}
		sa, err := c.Client.CoreV1().ServiceAccounts(mongodb.Namespace).Get(context.TODO(), saName, metav1.GetOptions{})
		if kerr.IsNotFound(err) {
			// create service account, since it does not exist
			if err = c.createServiceAccount(mongodb, saName); err != nil {
				if !kerr.IsAlreadyExists(err) {
					return err
				}
			}
		} else if err != nil {
			return err
		} else if _, controller := core_util.IsOwnedBy(sa, mongodb); !controller {
			// user provided the service account, so do nothing.
			return nil
		}

		// Create New Role
		pspName, err := c.getPolicyNames(mongodb)
		if err != nil {
			return err
		}
		if err = c.ensureRole(mongodb, mongodb.OffshootName(), pspName); err != nil {
			return err
		}

		// Create New RoleBinding
		if err = c.createRoleBinding(mongodb, mongodb.OffshootName(), saName); err != nil {
			return err
		}
		return nil
	}

	if mongodb.Spec.ShardTopology != nil {
		if err := createDatabaseRBAC(&mongodb.Spec.ShardTopology.ConfigServer.PodTemplate); err != nil {
			return err
		}
		if err := createDatabaseRBAC(&mongodb.Spec.ShardTopology.Mongos.PodTemplate); err != nil {
			return err
		}
		if err := createDatabaseRBAC(&mongodb.Spec.ShardTopology.Shard.PodTemplate); err != nil {
			return err
		}
	} else {
		if err := createDatabaseRBAC(mongodb.Spec.PodTemplate); err != nil {
			return err
		}
	}

	return nil
}
