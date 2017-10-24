package validator

import (
	"fmt"

	tapi "github.com/k8sdb/apimachinery/apis/kubedb/v1alpha1"
	"github.com/k8sdb/apimachinery/pkg/docker"
	amv "github.com/k8sdb/apimachinery/pkg/validator"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

func ValidateMongoDB(client kubernetes.Interface, mongodb *tapi.MongoDB) error {
	if mongodb.Spec.Version == "" {
		return fmt.Errorf(`Object 'Version' is missing in '%v'`, mongodb.Spec)
	}

	// Set Database Image version
	version := string(mongodb.Spec.Version)
	// TODO: docker.ImageMongoDB should hold correct image name
	if err := docker.CheckDockerImageVersion(docker.ImageMongoDB, version); err != nil {
		return fmt.Errorf(`Image %v:%v not found`, docker.ImageMongoDB, version)
	}

	if mongodb.Spec.Storage != nil {
		var err error
		if err = amv.ValidateStorage(client, mongodb.Spec.Storage); err != nil {
			return err
		}
	}

	// ---> Start
	// TODO: Use following if database needs/supports authentication secret
	// otherwise, delete
	databaseSecret := mongodb.Spec.DatabaseSecret
	if databaseSecret != nil {
		if _, err := client.CoreV1().Secrets(mongodb.Namespace).Get(databaseSecret.SecretName, metav1.GetOptions{}); err != nil {
			return err
		}
	}
	// ---> End

	backupScheduleSpec := mongodb.Spec.BackupSchedule
	if backupScheduleSpec != nil {
		if err := amv.ValidateBackupSchedule(client, backupScheduleSpec, mongodb.Namespace); err != nil {
			return err
		}
	}

	monitorSpec := mongodb.Spec.Monitor
	if monitorSpec != nil {
		if err := amv.ValidateMonitorSpec(monitorSpec); err != nil {
			return err
		}

	}
	return nil
}
