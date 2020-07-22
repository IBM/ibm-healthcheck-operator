//
// Copyright 2020 IBM Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HealthServiceSpecMemcached defines the desired state of HealthService.Memcached
type HealthServiceSpecMemcached struct {
	// memcached deployment name
	Name string `json:"name,"`
	// deprecated, define image in operator.yaml
	Image image `json:"image,omitempty"`
	// memcached deployment replicas, default is 0
	ReplicaCount int32 `json:"replicaCount,omitempty"`
	// memcached deployment ServiceAccountName, default is default
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// memcached deployment node selector, default is empty
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// memcached deployment tolerations, default is empty
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// memcached deployment security context, default is empty
	SecurityContext corev1.SecurityContext `json:"securityContext,omitempty"`
	// memcached startup command, default value is "memcached -m 64 -o modern -v"
	Command []string `json:"command,omitempty"`
	// resources defines the desired state of Resources
	Resources Resources `json:"resources,omitempty"`
}

// HealthServiceSpecHealthService defines the desired state of HealthService.HealthService
type HealthServiceSpecHealthService struct {
	// health service deployment name
	Name string `json:"name"`
	// deprecated, define image in operator.yaml
	Image image `json:"image,omitempty"`
	// configmap which contains health srevice configuration files, deprecated
	ConfigmapName string `json:"configmapName"`
	// set labels/annotation name to get pod's cloudpakname
	CloudpakNameSetting string `json:"cloudpakNameSetting,omitempty"`
	// set labels/annotation name to get pod's servicename
	ServiceNameSetting string `json:"serviceNameSetting,omitempty"`
	// set labels/annotation name to get pod's dependencies
	DependsSetting string `json:"dependsSetting,omitempty"`
	// health service deployment replicas, default is 0
	ReplicaCount int32 `json:"replicaCount,omitempty"`
	// health service deployment ServiceAccountName, default is default
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// health srevice deployment node selector, default is empty
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// health srevice deployment tolerations, default is empty
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// memcached deployment security context, default is empty
	SecurityContext corev1.SecurityContext `json:"securityContext,omitempty"`
	// health srevice deployment hostnetwork, default is false
	HostNetwork bool `json:"hostNetwork,omitempty"`
	// resources defines the desired state of Resources
	Resources Resources `json:"resources,omitempty"`
}

type Resource struct {
	Memory string `json:"memory,omitempty"`
	CPU    string `json:"cpu,omitempty"`
}

type Resources struct {
	Requests Resource `json:"requests,omitempty"`
	Limits   Resource `json:"limits,omitempty"`
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

	// +listType=set
	// MemcachedNodes are the names of the memcached pods
	MemcachedNodes []string `json:"memcachedNodes,omitempty"`
	// HealthCheckNodes are the names of the Healch Service pods
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
