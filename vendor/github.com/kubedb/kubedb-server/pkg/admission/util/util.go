package util

import (
	"fmt"
	"os"
	"strings"

	meta_util "github.com/appscode/kutil/meta"
	api "github.com/kubedb/apimachinery/apis/kubedb/v1alpha1"
	authenticationv1 "k8s.io/api/authentication/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/mergepatch"
	apiserver_util "k8s.io/apiserver/pkg/authentication/serviceaccount"
)

var (
	// SERVICE_ACCOUNT_NAME is the key of Environment key-value.
	// This key-value contains the name of service-account of KubeDB-Operator.
	// This environment will be set while deploying KubeDB-Server.
	EnvSvcAccountName = "SERVICE_ACCOUNT_NAME"
)

func IsKubeDBOperator(userInfo authenticationv1.UserInfo) bool {
	svcAccEnv := os.Getenv(EnvSvcAccountName)
	nsEnv := meta_util.Namespace()

	if ns, username, err := apiserver_util.SplitUsername(userInfo.Username); err == nil && username == svcAccEnv && ns == nsEnv {
		return true
	}
	return false
}

func ValidateUpdate(obj, oldObj runtime.Object, kind string) error {
	preconditions := getPreconditionFunc(kind)
	_, err := meta_util.CreateStrategicPatch(oldObj, obj, preconditions...)
	if err != nil {
		if mergepatch.IsPreconditionFailed(err) {
			return fmt.Errorf("%v.%v", err, preconditionFailedError(kind))
		}
		return err
	}
	return nil
}

func getPreconditionFunc(kind string) []mergepatch.PreconditionFunc {
	preconditions := []mergepatch.PreconditionFunc{
		mergepatch.RequireKeyUnchanged("apiVersion"),
		mergepatch.RequireKeyUnchanged("kind"),
		mergepatch.RequireMetadataKeyUnchanged("name"),
		mergepatch.RequireMetadataKeyUnchanged("namespace"),
	}

	if fields, found := preconditionSpecField[kind]; found {
		for _, field := range fields {
			preconditions = append(preconditions,
				meta_util.RequireChainKeyUnchanged(field),
			)
		}
	}
	return preconditions
}

var preconditionSpecField = map[string][]string{
	api.ResourceKindElasticsearch: {
		"spec.version",
		"spec.topology.*.prefix",
		"spec.enableSSL",
		"spec.certificateSecret",
		"spec.databaseSecret",
		"spec.storage",
		"spec.nodeSelector",
		"spec.init",
	},
	api.ResourceKindPostgres: {
		"spec.version",
		"spec.standby",
		"spec.streaming",
		"spec.archiver",
		"spec.databaseSecret",
		"spec.storage",
		"spec.nodeSelector",
		"spec.init",
	},
	api.ResourceKindMySQL: {
		"spec.version",
		"spec.storage",
		"spec.databaseSecret",
		"spec.nodeSelector",
		"spec.init",
	},
	api.ResourceKindMongoDB: {
		"spec.version",
		"spec.storage",
		"spec.databaseSecret",
		"spec.nodeSelector",
		"spec.init",
	},
	api.ResourceKindRedis: {
		"spec.version",
		"spec.storage",
		"spec.nodeSelector",
	},
	api.ResourceKindMemcached: {
		"spec.version",
		"spec.nodeSelector",
	},
	api.ResourceKindDormantDatabase: {
		"spec.origin",
	},
}

func preconditionFailedError(kind string) error {
	str := preconditionSpecField[kind]
	strList := strings.Join(str, "\n\t")
	return fmt.Errorf(strings.Join([]string{`At least one of the following was changed:
	apiVersion
	kind
	name
	namespace
	status`, strList}, "\n\t"))
}
