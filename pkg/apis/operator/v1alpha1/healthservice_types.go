package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// image defines the desired image repository, tag and imagepullpolicy
type image struct {
	// image repository, default is empty
	Repository string `json:"repository"`
	// image tag, default is empty
	Tag string `json:"tag"`
	// image pull policy, default is IfNotPresent
	PullPolicy string `json:"pullPolicy,omitempty"`
}

// resource defines the desired resource requests and limits of memory and cpu
type resources struct {
	// resource requests of memory, default is empty
	RequestsMemory string `json:"requestsMemory,omitempty"`
	// resource requests of cpu, default is empty
	RequestsCPU string `json:"requestsCpu,omitempty"`
	// resource limits of memory, default is empty
	LimitsMemory string `json:"limitsMemory,omitempty"`
	// resource limits of cpu, default is empty
	LimitsCPU string `json:"limitsCpu,omitempty"`
}

// HealthServiceSpecMemcached defines the desired state of HealthService.Memcached
type HealthServiceSpecMemcached struct {
	// memcached deployment name
	Name string `json:"name,"`
	// memcached service name
	ServiceName string `json:"serviceName"`
	// memcached image repository, tag and imagepullpolicy
	Image image `json:"image,"`
	// memcached deployment replicas, default is 0
	ReplicaCount int32 `json:"replicaCount,omitempty"`
	// memcached deployment node selector, default is empty
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// memcached deployment tolerations, default is empty
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// memcached deployment security context, default is empty
	SecurityContext corev1.SecurityContext `json:"securityContext,omitempty"`
	// memcached startup command, default value is "memcached -m 64 -o modern -v"
	Command []string `json:"command,omitempty"`
}

// HealthServiceSpecHealthService defines the desired state of HealthService.HealthService
type HealthServiceSpecHealthService struct {
	// health service deployment name
	Name string `json:"name"`
	// health service image repository, tag and imagepullpolicy
	Image image `json:"image"`
	// configmap which contains health srevice configuration files, deprecated
	ConfigmapName string `json:"configmapName"`
	// health service deployment replicas, default is 0
	ReplicaCount int32 `json:"replicaCount,omitempty"`
	// health srevice deployment node selector, default is empty
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// health srevice deployment tolerations, default is empty
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// memcached deployment security context, default is empty
	SecurityContext corev1.SecurityContext `json:"securityContext,omitempty"`
	// health srevice deployment hostnetwork, default is false
	HostNetwork bool `json:"hostNetwork,omitempty"`
}

// HealthServiceSpec defines the desired state of HealthService
type HealthServiceSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// Memcached defines the desired state of HealthService.Memcached
	Memcached HealthServiceSpecMemcached `json:"memcached,omitempty"`
	// HealthService defines the desired state of HealthService.HealthService
	HealthService HealthServiceSpecHealthService `json:"healthService,omitempty"`
}

// HealthServiceStatus defines the observed state of HealthService
type HealthServiceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html

	// MemcachedNodes are the names of the memcached pods
	// +listType=set
	MemcachedNodes   []string `json:"memcachedNodes,omitempty"`
	HealthCheckNodes []string `json:"healthCheckNodes,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HealthService is the Schema for the healthservices API
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=healthservices,scope=Namespaced
type HealthService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HealthServiceSpec   `json:"spec,omitempty"`
	Status HealthServiceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HealthServiceList contains a list of HealthService
type HealthServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HealthService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&HealthService{}, &HealthServiceList{})
}
