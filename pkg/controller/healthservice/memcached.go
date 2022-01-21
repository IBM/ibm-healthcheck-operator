//
// Copyright 2020, 2021 IBM Corporation
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

package healthservice

import (
	"context"
	"os"
	"reflect"

	operatorv1alpha1 "github.com/IBM/ibm-healthcheck-operator/pkg/apis/operator/v1alpha1"
	common "github.com/IBM/ibm-healthcheck-operator/pkg/controller/common"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var memSvcName = "memcached"
var memResourceName = "icp-memcached"

var trueVar = true
var falseVar = false
var commonSecurityContext = corev1.SecurityContext{
	AllowPrivilegeEscalation: &falseVar,
	Privileged:               &falseVar,
	ReadOnlyRootFilesystem:   &trueVar,
	RunAsNonRoot:             &trueVar,
	Capabilities: &corev1.Capabilities{
		Drop: []corev1.Capability{
			"ALL",
		},
	},
}

func (r *ReconcileHealthService) createOrUpdateMemcachedDeploy(h *operatorv1alpha1.HealthService) error {
	memName := memResourceName
	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)

	// Define a new deployment
	desired := r.desiredMemcachedDeployment(h)
	// Check if the deployment already exists, if not create a new one
	current := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: memName, Namespace: h.Namespace}, current)

	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", desired.Namespace, "Deployment.Name", desired.Name)
		if err := r.client.Create(context.TODO(), desired); err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", desired.Namespace, "Deployment.Name", desired.Name)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment", "Deployment.Namespace", current.Namespace, "Deployment.Name", current.Name)
		return err
	} else if err := r.updateMemcachedDeployment(h, current, desired); err != nil {
		return err
	}

	// Update the HealthService status with the pod names
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(h.Namespace),
		client.MatchingLabels(labelsForMemcached(memName, h.Name)),
	}
	if err = r.client.List(context.TODO(), podList, listOpts...); err != nil {
		reqLogger.Error(err, "Failed to list pods", "h.Namespace", h.Namespace, "h.Name", memName)
		return err
	}
	podNames := common.GetPodNames(podList.Items)

	// Update status.MemcachedNodes if needed
	if !reflect.DeepEqual(podNames, h.Status.MemcachedNodes) {
		h.Status.MemcachedNodes = podNames
		err := r.client.Status().Update(context.TODO(), h)
		if err != nil {
			reqLogger.Error(err, "Failed to update HealthService status")
			return err
		}
	}

	return nil
}

func (r *ReconcileHealthService) createOrUpdateMemcachedService(h *operatorv1alpha1.HealthService) error {
	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)

	// Define a new service
	desired := r.desiredMemcachedService(h)
	// Check if the service already exists, if not create a new one
	current := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: memSvcName, Namespace: h.Namespace}, current)

	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Service", "Service.Namespace", desired.Namespace, "Service.Name", desired.Name)
		if err := r.client.Create(context.TODO(), desired); err != nil {
			reqLogger.Error(err, "Failed to create new Service", "Service.Namespace", desired.Namespace, "Service.Name", desired.Name)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Service", "Service.Namespace", current.Namespace, "Service.Name", current.Name)
		return err
	} else if err := r.updateMemcachedService(h, current, desired); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileHealthService) updateMemcachedDeployment(h *operatorv1alpha1.HealthService, current, desired *appsv1.Deployment) error {
	reqLogger := log.WithValues("Deployment.Namespace", current.Namespace, "Deployment.Name", current.Name)

	updated := current.DeepCopy()
	updated.Spec.Replicas = desired.Spec.Replicas
	updated.Spec.Template.ObjectMeta.Annotations = desired.Spec.Template.ObjectMeta.Annotations
	updated.Spec.Template.Spec.Containers = desired.Spec.Template.Spec.Containers

	reqLogger.Info("Updating Deployment")
	// Set HealthService instance as the owner and controller
	if err := controllerutil.SetControllerReference(h, updated, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Deployment.Namespace", updated.Namespace, "Deployment.Name", updated.Name)
	}

	if err := r.client.Update(context.TODO(), updated); err != nil {
		reqLogger.Error(err, "Failed to update Deployment", "Deployment.Namespace", updated.Namespace, "Deployment.Name", updated.Name)
		return err
	}

	return nil
}

func (r *ReconcileHealthService) desiredMemcachedDeployment(h *operatorv1alpha1.HealthService) *appsv1.Deployment {
	memName := memResourceName
	labels := labelsForMemcached(memName, h.Name)
	annotations := annotationsForMemcached()
	defaultCommand := []string{"memcached", "-m 64", "-o", "modern", "-v"}
	serviceAccountName := "ibm-healthcheck-operator-cluster"
	if h.Spec.Memcached.Command != nil && len(h.Spec.Memcached.Command) > 0 {
		defaultCommand = h.Spec.Memcached.Command
	}

	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)
	reqLogger.Info("Building Memcached Deployment", "Deployment.Namespace", h.Namespace, "Deployment.Name", memName)

	hmResources := common.GetResources(&h.Spec.Memcached.Resources)
	hmReplicas := int32(1)
	if h.Spec.Memcached.Replicas > 0 {
		hmReplicas = h.Spec.Memcached.Replicas
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      memName,
			Namespace: h.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			// Replicas: &h.Spec.Memcached.ReplicaCount,
			Replicas: &hmReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					HostNetwork:        false,
					HostPID:            false,
					HostIPC:            false,
					ServiceAccountName: serviceAccountName,
					Containers: []corev1.Container{{
						Name:            memName,
						Image:           os.Getenv("ICP_MEMCACHED_IMAGE"),
						ImagePullPolicy: corev1.PullIfNotPresent,
						Command:         defaultCommand,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 11211,
							Name:          memName,
						}},
						SecurityContext: &commonSecurityContext,
						LivenessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.FromString(memName),
								},
							},
							InitialDelaySeconds: 30,
							TimeoutSeconds:      5,
						},
						ReadinessProbe: &corev1.Probe{
							Handler: corev1.Handler{
								TCPSocket: &corev1.TCPSocketAction{
									Port: intstr.IntOrString{Type: intstr.String, StrVal: memName},
								},
							},
							InitialDelaySeconds: 5,
							TimeoutSeconds:      1,
						},
						Resources: *hmResources,
					}},
					Tolerations: []corev1.Toleration{
						{
							Key:      "dedicated",
							Operator: corev1.TolerationOpExists,
							Effect:   corev1.TaintEffectNoSchedule,
						},
						{
							Key:      "CriticalAddonsOnly",
							Operator: corev1.TolerationOpExists,
						},
					},
				},
			},
		},
	}

	// Set HealthService instance as the owner and controller
	if err := controllerutil.SetControllerReference(h, dep, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Deployment.Namespace", h.Namespace, "Deployment.Name", memName)
	}

	return dep
}

func (r *ReconcileHealthService) updateMemcachedService(h *operatorv1alpha1.HealthService, current, desired *corev1.Service) error {
	reqLogger := log.WithValues("Service.Namespace", current.Namespace, "Service.Name", current.Name)

	updated := current.DeepCopy()
	updated.ObjectMeta.Labels = desired.ObjectMeta.Labels
	updated.Spec.Ports = desired.Spec.Ports
	updated.Spec.Selector = desired.Spec.Selector
	updated.Spec.ClusterIP = desired.Spec.ClusterIP

	reqLogger.Info("Updating Service")
	// Set HealthService instance as the owner and controller
	if err := controllerutil.SetControllerReference(h, updated, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Service.Namespace", updated.Namespace, "Service.Name", updated.Name)
	}

	if err := r.client.Update(context.TODO(), updated); err != nil {
		reqLogger.Error(err, "Failed to update Service", "Service.Namespace", updated.Namespace, "Service.Name", updated.Name)
		return err
	}

	return nil
}

func (r *ReconcileHealthService) desiredMemcachedService(h *operatorv1alpha1.HealthService) *corev1.Service {
	memName := memResourceName
	labels := labelsForMemcached(memName, h.Name)

	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)
	reqLogger.Info("Building Memcached Service", "Service.Namespace", h.Namespace, "Service.Name", memSvcName)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            memSvcName,
			Namespace:       h.Namespace,
			Labels:          labels,
			ResourceVersion: "",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Name:       memName,
					Port:       11211,
					TargetPort: intstr.IntOrString{Type: intstr.String, StrVal: memName},
				},
			},
			Selector:  labels,
			ClusterIP: "None",
		},
	}

	// Set HealthService instance as the owner and controller
	if err := controllerutil.SetControllerReference(h, svc, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Service.Namespace", h.Namespace, "Service.Name", memSvcName)
	}

	return svc
}

func labelsForMemcached(name, releaseName string) map[string]string {
	return map[string]string{
		"app":                          name,
		"release":                      releaseName,
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/instance":   releaseName,
		"app.kubernetes.io/managed-by": "",
	}
}

func annotationsForMemcached() map[string]string {
	return map[string]string{
		"productName":   "IBM Cloud Platform Common Services",
		"productID":     "068a62892a1e4db39641342e592daa25",
		"productMetric": "FREE",
	}
}
