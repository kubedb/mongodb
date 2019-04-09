package controller

import (
	"fmt"
	"strconv"

	"github.com/appscode/go/types"
	"github.com/fatih/structs"
	catalog "github.com/kubedb/apimachinery/apis/catalog/v1alpha1"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/kubedb/apimachinery/pkg/eventer"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/reference"
	kutil "kmodules.xyz/client-go"
	app_util "kmodules.xyz/client-go/apps/v1"
	core_util "kmodules.xyz/client-go/core/v1"
	meta_util "kmodules.xyz/client-go/meta"
	mona "kmodules.xyz/monitoring-agent-api/api/v1"
	ofst "kmodules.xyz/offshoot-api/api/v1"
)

func (c *Controller) checkDeployment(mongodb *api.MongoDB, deployName string) error {
	// Deployment for MongoDB database
	deployment, err := c.Client.AppsV1().Deployments(mongodb.Namespace).Get(deployName, metav1.GetOptions{})
	if err != nil {
		if kerr.IsNotFound(err) {
			return nil
		}
		return err
	}
	if deployment.Labels[api.LabelDatabaseKind] != api.ResourceKindMongoDB ||
		deployment.Labels[api.LabelDatabaseName] != mongodb.Name {
		return fmt.Errorf(`intended deployment "%v/%v" already exists`, mongodb.Namespace, deployName)
	}
	return nil
}

func (c *Controller) ensureDeployment(
	mongodb *api.MongoDB,
	stsName string,
	replicas *int32,
	labels map[string]string,
	selectors map[string]string,
	cmd []string, // cmd of `mongodb` container
	args []string, // args of `mongodb` container
	envList []core.EnvVar, // envList of `mongodb` container
	podTemplate *ofst.PodTemplateSpec,
	volumeMount []core.VolumeMount,
	strategy apps.DeploymentStrategy,
	initContainers []core.Container,
	volume []core.Volume, // volumes to mount on stsPodTemplate
) (kutil.VerbType, error) {
	// Take value of podTemplate
	var pt ofst.PodTemplateSpec
	if podTemplate != nil {
		pt = *podTemplate
	}
	if err := c.checkDeployment(mongodb, stsName); err != nil {
		return kutil.VerbUnchanged, err
	}
	deploymentMeta := metav1.ObjectMeta{
		Name:      stsName,
		Namespace: mongodb.Namespace,
	}

	ref, rerr := reference.GetReference(clientsetscheme.Scheme, mongodb)
	if rerr != nil {
		return kutil.VerbUnchanged, rerr
	}

	mongodbVersion, err := c.ExtClient.CatalogV1alpha1().MongoDBVersions().Get(string(mongodb.Spec.Version), metav1.GetOptions{})
	if err != nil {
		return kutil.VerbUnchanged, err
	}

	readinessProbe := pt.Spec.ReadinessProbe
	if readinessProbe != nil && structs.IsZero(*readinessProbe) {
		readinessProbe = nil
	}
	livenessProbe := pt.Spec.LivenessProbe
	if livenessProbe != nil && structs.IsZero(*livenessProbe) {
		livenessProbe = nil
	}

	deployment, vt, err := app_util.CreateOrPatchDeployment(c.Client, deploymentMeta, func(in *apps.Deployment) *apps.Deployment {
		in.Labels = labels
		in.Annotations = pt.Controller.Annotations
		core_util.EnsureOwnerReference(&in.ObjectMeta, ref)

		in.Spec.Replicas = replicas
		in.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: selectors,
		}
		in.Spec.Template.Labels = selectors
		in.Spec.Template.Annotations = pt.Annotations
		in.Spec.Template.Spec.InitContainers = core_util.UpsertContainers(
			in.Spec.Template.Spec.InitContainers, pt.Spec.InitContainers,
		)
		in.Spec.Template.Spec.Containers = core_util.UpsertContainer(
			in.Spec.Template.Spec.Containers,
			core.Container{
				Name:            api.ResourceSingularMongoDB,
				Image:           mongodbVersion.Spec.DB.Image,
				ImagePullPolicy: core.PullIfNotPresent,
				Command:         cmd,
				Args: meta_util.UpsertArgumentList(
					args, pt.Spec.Args),
				Ports: []core.ContainerPort{
					{
						Name:          "db",
						ContainerPort: MongoDBPort,
						Protocol:      core.ProtocolTCP,
					},
				},
				Env:            core_util.UpsertEnvVars(envList, pt.Spec.Env...),
				Resources:      pt.Spec.Resources,
				Lifecycle:      pt.Spec.Lifecycle,
				LivenessProbe:  livenessProbe,
				ReadinessProbe: readinessProbe,
				VolumeMounts:   volumeMount,
			})

		in.Spec.Template.Spec.InitContainers = core_util.UpsertContainers(
			in.Spec.Template.Spec.InitContainers,
			initContainers,
		)

		if mongodb.GetMonitoringVendor() == mona.VendorPrometheus {
			in.Spec.Template.Spec.Containers = core_util.UpsertContainer(
				in.Spec.Template.Spec.Containers,
				core.Container{
					Name: "exporter",
					Args: append([]string{
						fmt.Sprintf("--web.listen-address=:%d", mongodb.Spec.Monitor.Prometheus.Port),
						fmt.Sprintf("--web.metrics-path=%v", mongodb.StatsService().Path()),
						"--mongodb.uri=mongodb://$(MONGO_INITDB_ROOT_USERNAME):$(MONGO_INITDB_ROOT_PASSWORD)@127.0.0.1:27017",
					}, mongodb.Spec.Monitor.Args...),
					Image: mongodbVersion.Spec.Exporter.Image,
					Ports: []core.ContainerPort{
						{
							Name:          api.PrometheusExporterPortName,
							Protocol:      core.ProtocolTCP,
							ContainerPort: mongodb.Spec.Monitor.Prometheus.Port,
						},
					},
					Env:             mongodb.Spec.Monitor.Env,
					Resources:       mongodb.Spec.Monitor.Resources,
					SecurityContext: mongodb.Spec.Monitor.SecurityContext,
				},
			)
		}

		in.Spec.Template.Spec.Volumes = core_util.UpsertVolume(
			in.Spec.Template.Spec.Volumes,
			volume...,
		)

		in.Spec.Template.Spec.Volumes = core_util.UpsertVolume(in.Spec.Template.Spec.Volumes, core.Volume{
			Name: configDirectoryName,
			VolumeSource: core.VolumeSource{
				EmptyDir: &core.EmptyDirVolumeSource{},
			},
		})
		in.Spec.Template = upsertEnv(in.Spec.Template, mongodb)

		if mongodb.Spec.ConfigSource != nil {
			in.Spec.Template = c.upsertConfigSourceVolume(in.Spec.Template, mongodb)
		}

		in.Spec.Template.Spec.NodeSelector = pt.Spec.NodeSelector
		in.Spec.Template.Spec.Affinity = pt.Spec.Affinity
		if pt.Spec.SchedulerName != "" {
			in.Spec.Template.Spec.SchedulerName = pt.Spec.SchedulerName
		}
		in.Spec.Template.Spec.Tolerations = pt.Spec.Tolerations
		in.Spec.Template.Spec.ImagePullSecrets = pt.Spec.ImagePullSecrets
		in.Spec.Template.Spec.PriorityClassName = pt.Spec.PriorityClassName
		in.Spec.Template.Spec.Priority = pt.Spec.Priority
		in.Spec.Template.Spec.SecurityContext = pt.Spec.SecurityContext

		if c.EnableRBAC {
			in.Spec.Template.Spec.ServiceAccountName = mongodb.OffshootName()
		}

		in.Spec.Strategy = strategy

		return in
	})

	if err != nil {
		return kutil.VerbUnchanged, err
	}

	// Check StatefulSet Pod status
	if vt != kutil.VerbUnchanged {
		if err := app_util.WaitUntilDeploymentReady(c.Client, deployment.ObjectMeta); err != nil {
			return kutil.VerbUnchanged, err
		}
		c.recorder.Eventf(
			mongodb,
			core.EventTypeNormal,
			eventer.EventReasonSuccessful,
			"Successfully %v Deployment %v/%v",
			vt, mongodb.Namespace, stsName,
		)
	}
	return vt, nil
}

func (c *Controller) ensureMongosNode(mongodb *api.MongoDB) (kutil.VerbType, error) {
	mongodbVersion, err := c.ExtClient.CatalogV1alpha1().MongoDBVersions().Get(string(mongodb.Spec.Version), metav1.GetOptions{})
	if err != nil {
		return kutil.VerbUnchanged, err
	}

	cmds := []string{"mongos"}
	args := []string{
		"--bind_ip=0.0.0.0",
		"--port=" + strconv.Itoa(MongoDBPort),
		"--configdb=$(CONFIGDB_REPSET)",
		"--clusterAuthMode=keyFile",
		"--keyFile=" + configDirectoryPath + "/" + KeyForKeyFile,
	}

	// shardDsn List, separated by space ' '
	var shardDsn string
	for i := int32(0); i < mongodb.Spec.ShardTopology.Shard.Shards; i++ {
		if i != 0 {
			shardDsn += " "
		}
		shardDsn += mongodb.ShardDSN(i)
	}

	envList := []core.EnvVar{
		{
			Name:  "CONFIGDB_REPSET",
			Value: mongodb.ConfigSvrDSN(),
		},
		{
			Name:  "SHARD_REPSETS",
			Value: shardDsn,
		},
	}

	initContnr, initvolumes := installInitContainer(
		mongodb,
		mongodbVersion,
		&mongodb.Spec.ShardTopology.Mongos.PodTemplate,
	)

	var initContainers []core.Container
	var volumes []core.Volume

	volumeMounts := []core.VolumeMount{
		{
			Name:      configDirectoryName,
			MountPath: configDirectoryPath,
		},
	}

	initContainers = append(initContainers, initContnr)
	volumes = append(volumes, initvolumes)

	if mongodb.Spec.Init != nil && mongodb.Spec.Init.ScriptSource != nil {
		volumes = append(volumes, core.Volume{
			Name:         "initial-script",
			VolumeSource: mongodb.Spec.Init.ScriptSource.VolumeSource,
		})

		volumeMounts = append(
			volumeMounts,
			[]core.VolumeMount{
				{
					Name:      "initial-script",
					MountPath: "/docker-entrypoint-initdb.d",
				},
			}...,
		)
	}

	bootstrpContnr, bootstrpVol := mongosInitContainer(
		mongodb,
		mongodbVersion,
		mongodb.Spec.ShardTopology.Mongos.PodTemplate,
		envList,
		"mongos.sh",
	)
	initContainers = append(initContainers, bootstrpContnr)
	volumes = append(volumes, bootstrpVol...)

	return c.ensureDeployment(
		mongodb,
		mongodb.MongosNodeName(),
		&mongodb.Spec.ShardTopology.Mongos.Replicas,
		mongodb.MongosLabels(),
		mongodb.MongosSelectors(),
		cmds,
		args,
		envList,
		&mongodb.Spec.ShardTopology.Mongos.PodTemplate,
		volumeMounts,
		mongodb.Spec.ShardTopology.Mongos.Strategy,
		initContainers,
		volumes,
	)
}

func mongosInitContainer(
	mongodb *api.MongoDB,
	mongodbVersion *catalog.MongoDBVersion,
	podTemplate ofst.PodTemplateSpec,
	envList []core.EnvVar,
	scriptName string,
) (core.Container, []core.Volume) {

	envList = core_util.UpsertEnvVars(envList, podTemplate.Spec.Env...)

	bootstrapContainer := core.Container{
		Name:            InitBootstrapContainerName,
		Image:           mongodbVersion.Spec.DB.Image,
		ImagePullPolicy: core.PullIfNotPresent,
		Command:         []string{"/bin/sh"},
		Args: []string{
			"-c",
			fmt.Sprintf("/usr/local/bin/%v", scriptName),
		},
		Env: core_util.UpsertEnvVars([]core.EnvVar{
			{
				Name:  "AUTH",
				Value: "true",
			},
			{
				Name: "MONGO_INITDB_ROOT_USERNAME",
				ValueFrom: &core.EnvVarSource{
					SecretKeyRef: &core.SecretKeySelector{
						LocalObjectReference: core.LocalObjectReference{
							Name: mongodb.Spec.DatabaseSecret.SecretName,
						},
						Key: KeyMongoDBUser,
					},
				},
			},
			{
				Name: "MONGO_INITDB_ROOT_PASSWORD",
				ValueFrom: &core.EnvVarSource{
					SecretKeyRef: &core.SecretKeySelector{
						LocalObjectReference: core.LocalObjectReference{
							Name: mongodb.Spec.DatabaseSecret.SecretName,
						},
						Key: KeyMongoDBPassword,
					},
				},
			},
		}, envList...),
		VolumeMounts: []core.VolumeMount{
			{
				Name:      workDirectoryName,
				MountPath: workDirectoryPath,
			},
			{
				Name:      configDirectoryName,
				MountPath: configDirectoryPath,
			},
		},
	}

	rsVolume := []core.Volume{
		{
			Name: initialKeyDirectoryName,
			VolumeSource: core.VolumeSource{
				Secret: &core.SecretVolumeSource{
					DefaultMode: types.Int32P(256),
					SecretName:  mongodb.Spec.CertificateSecret.SecretName,
				},
			},
		},
	}

	if mongodb.Spec.Init != nil && mongodb.Spec.Init.ScriptSource != nil {
		rsVolume = append(rsVolume, core.Volume{
			Name:         "initial-script",
			VolumeSource: mongodb.Spec.Init.ScriptSource.VolumeSource,
		})

		bootstrapContainer.VolumeMounts = core_util.UpsertVolumeMount(
			bootstrapContainer.VolumeMounts,
			core.VolumeMount{
				Name:      "initial-script",
				MountPath: "/docker-entrypoint-initdb.d",
			},
		)
	}

	return bootstrapContainer, rsVolume
}
