package v1alpha1

import (
	"github.com/appscode/go/encoding/json/types"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ResourceCodeMySQL = "ms"
	ResourceKindMySQL = "MySQL"
	ResourceNameMySQL = "mysql"
	ResourceTypeMySQL = "mysqls"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Mysql defines a Mysql database.
type MySQL struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              MySQLSpec   `json:"spec,omitempty"`
	Status            MySQLStatus `json:"status,omitempty"`
}

type MySQLSpec struct {
	// Version of MongoDB to be deployed.
	Version types.StrYo `json:"version,omitempty"`
	// Number of instances to deploy for a MongoDB database.
	Replicas int32 `json:"replicas,omitempty"`
	// Storage spec to specify how storage shall be used.
	Storage *core.PersistentVolumeClaimSpec `json:"storage,omitempty"`
	// Database authentication secret
	DatabaseSecret *core.SecretVolumeSource `json:"databaseSecret,omitempty"`
	// NodeSelector is a selector which must be true for the pod to fit on a node
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Init is used to initialize database
	// +optional
	Init *InitSpec `json:"init,omitempty"`
	// BackupSchedule spec to specify how database backup will be taken
	// +optional
	BackupSchedule *BackupScheduleSpec `json:"backupSchedule,omitempty"`
	// If DoNotPause is true, controller will prevent to delete this Mysql object.
	// Controller will create same Mysql object and ignore other process.
	// +optional
	DoNotPause bool `json:"doNotPause,omitempty"`
	// Monitor is used monitor database instance
	// +optional
	Monitor *MonitorSpec `json:"monitor,omitempty"`
	// Compute Resources required by the sidecar container.
	Resources core.ResourceRequirements `json:"resources,omitempty"`
	// If specified, the pod's scheduling constraints
	// +optional
	Affinity *core.Affinity `json:"affinity,omitempty" protobuf:"bytes,18,opt,name=affinity"`
	// If specified, the pod will be dispatched by specified scheduler.
	// If not specified, the pod will be dispatched by default scheduler.
	// +optional
	SchedulerName string `json:"schedulerName,omitempty" protobuf:"bytes,19,opt,name=schedulerName"`
	// If specified, the pod's tolerations.
	// +optional
	Tolerations []core.Toleration `json:"tolerations,omitempty" protobuf:"bytes,22,opt,name=tolerations"`
}

type MySQLStatus struct {
	CreationTime *metav1.Time  `json:"creationTime,omitempty"`
	Phase        DatabasePhase `json:"phase,omitempty"`
	Reason       string        `json:"reason,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type MySQLList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is a list of MySQL TPR objects
	Items []MySQL `json:"items,omitempty"`
}
