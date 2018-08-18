package controller

import (
	"github.com/appscode/go/types"
	core_util "github.com/appscode/kutil/core/v1"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/client/clientset/versioned/typed/kubedb/v1alpha1/util"
	"github.com/kubedb/apimachinery/pkg/eventer"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
)

const (
	ConfigSourceSuffix = "-conf"
	ConfigSourceName   = "mongod.conf"
)

var (
	mongodbConf = "null"
)

func (c *Controller) upsertConfigSourceVolume(statefulSet *apps.StatefulSet, mongodb *api.MongoDB) *apps.StatefulSet {
	for i, container := range statefulSet.Spec.Template.Spec.Containers {
		if container.Name == api.ResourceSingularMongoDB {
			args := sets.NewString(statefulSet.Spec.Template.Spec.Containers[i].Args...)
			args.Insert("--config=" + configDirectoryPath + "/mongod.conf")
			statefulSet.Spec.Template.Spec.Containers[i].Args = args.List()
		}
	}

	for i, container := range statefulSet.Spec.Template.Spec.InitContainers {
		if container.Name == InitInstallContainerName {

			volumeMounts := []core.VolumeMount{
				{
					Name:      initialConfigDirectoryName,
					MountPath: initialConfigDirectoryPath,
				},
			}
			statefulSet.Spec.Template.Spec.InitContainers[i].VolumeMounts = core_util.UpsertVolumeMount(
				statefulSet.Spec.Template.Spec.InitContainers[i].VolumeMounts, volumeMounts...)
		}
	}

	volumes := core.Volume{

		Name:         initialConfigDirectoryName,
		VolumeSource: *mongodb.Spec.ConfigSource,
	}
	statefulSet.Spec.Template.Spec.Volumes = core_util.UpsertVolume(statefulSet.Spec.Template.Spec.Volumes, volumes)

	return statefulSet
}

func (c *Controller) ensureConfigMap(mongodb *api.MongoDB) error {
	if mongodb.Spec.ConfigSource == nil {
		configVolumeSource, err := c.createConfigSource(mongodb)
		if err != nil {
			if ref, rerr := reference.GetReference(clientsetscheme.Scheme, mongodb); rerr == nil {
				c.recorder.Eventf(
					ref,
					core.EventTypeWarning,
					eventer.EventReasonFailedToCreate,
					`Failed to create Config file (for mongodb.conf). Reason: %v`,
					err.Error(),
				)
			}
			return err
		}
		ms, _, err := util.PatchMongoDB(c.ExtClient, mongodb, func(in *api.MongoDB) *api.MongoDB {
			in.Spec.ConfigSource = &core.VolumeSource{
				ConfigMap: configVolumeSource,
			}
			return in
		})
		if err != nil {
			if ref, rerr := reference.GetReference(clientsetscheme.Scheme, mongodb); rerr == nil {
				c.recorder.Eventf(
					ref,
					core.EventTypeWarning,
					eventer.EventReasonFailedToUpdate,
					err.Error(),
				)
			}
			return err
		}
		mongodb.Spec.ConfigSource = ms.Spec.ConfigSource
	}
	return nil
}

func (c *Controller) createConfigSource(mongodb *api.MongoDB) (*core.ConfigMapVolumeSource, error) {
	configMapName := mongodb.Name + ConfigSourceSuffix

	configMap := &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: mongodb.Namespace,
			Labels:    mongodb.OffshootLabels(),
		},
		Data: map[string]string{
			ConfigSourceName: mongodbConf,
		},
	}

	_, err := c.Client.CoreV1().ConfigMaps(mongodb.Namespace).Create(configMap)
	if err != nil && !kerr.IsAlreadyExists(err) {
		return nil, err
	}

	return &core.ConfigMapVolumeSource{
		LocalObjectReference: core.LocalObjectReference{
			Name: configMap.Name,
		},
		DefaultMode: types.Int32P(420),
	}, nil
}
