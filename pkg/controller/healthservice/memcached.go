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

package healthservice

import (
	"context"
	"reflect"

	operatorv1alpha1 "github.com/IBM/ibm-healthcheck-operator/pkg/apis/operator/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	intstr "k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var cpu50 = resource.NewMilliQuantity(50, resource.DecimalSI)          // 50m
var cpu500 = resource.NewMilliQuantity(500, resource.DecimalSI)        // 500m
var memory64 = resource.NewQuantity(64*1024*1024, resource.BinarySI)   // 64Mi
var memory128 = resource.NewQuantity(128*1024*1024, resource.BinarySI) // 128Mi
var memSvcName = "memcached"

func (r *ReconcileHealthService) createOrUpdateMemcachedDeploy(h *operatorv1alpha1.HealthService) error {
	memName := h.Spec.Memcached.Name
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
	podNames := getPodNames(podList.Items)

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
	updated.ObjectMeta.Labels = desired.ObjectMeta.Labels
	updated.Spec.Replicas = desired.Spec.Replicas
	updated.Spec.Selector.MatchLabels = desired.Spec.Selector.MatchLabels
	updated.Spec.Template.ObjectMeta.Labels = desired.Spec.Template.ObjectMeta.Labels
	updated.Spec.Template.ObjectMeta.Annotations = desired.Spec.Template.ObjectMeta.Annotations
	updated.Spec.Template.Spec.Containers = desired.Spec.Template.Spec.Containers
	updated.Spec.Template.Spec.ServiceAccountName = desired.Spec.Template.Spec.ServiceAccountName
	updated.Spec.Template.Spec.HostNetwork = desired.Spec.Template.Spec.HostNetwork
	updated.Spec.Template.Spec.HostPID = desired.Spec.Template.Spec.HostPID
	updated.Spec.Template.Spec.HostIPC = desired.Spec.Template.Spec.HostIPC
	updated.Spec.Template.Spec.NodeSelector = desired.Spec.Template.Spec.NodeSelector
	updated.Spec.Template.Spec.Tolerations = desired.Spec.Template.Spec.Tolerations

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
	memName := h.Spec.Memcached.Name
	labels := labelsForMemcached(memName, h.Name)
	annotations := annotationsForMemcached()
	defaultCommand := []string{"memcached", "-m 64", "-o", "modern", "-v"}
	serviceAccountName := "default"
	if h.Spec.Memcached.Command != nil && len(h.Spec.Memcached.Command) > 0 {
		defaultCommand = h.Spec.Memcached.Command
	}
	if len(h.Spec.Memcached.ServiceAccountName) > 0 {
		serviceAccountName = h.Spec.Memcached.ServiceAccountName
	}

	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)
	reqLogger.Info("Building Memcached Deployment", "Deployment.Namespace", h.Namespace, "Deployment.Name", memName)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      memName,
			Namespace: h.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &h.Spec.Memcached.ReplicaCount,
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
						Image:           h.Spec.Memcached.Image.Repository + "/" + h.Spec.Memcached.Image.Name + ":" + h.Spec.Memcached.Image.Tag,
						ImagePullPolicy: corev1.PullPolicy(h.Spec.Memcached.Image.PullPolicy),
						Command:         defaultCommand,
						Ports: []corev1.ContainerPort{{
							ContainerPort: 11211,
							Name:          memName,
						}},
						SecurityContext: &h.Spec.Memcached.SecurityContext,
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
						Resources: corev1.ResourceRequirements{
							Limits: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    *cpu500,
								corev1.ResourceMemory: *memory128},
							Requests: map[corev1.ResourceName]resource.Quantity{
								corev1.ResourceCPU:    *cpu50,
								corev1.ResourceMemory: *memory64},
						},
					}},
					NodeSelector: h.Spec.Memcached.NodeSelector,
					Tolerations:  h.Spec.Memcached.Tolerations,
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
	memName := h.Spec.Memcached.Name
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

// getPodNames returns the pod names of the array of pods passed in
func getPodNames(pods []corev1.Pod) []string {
	var podNames []string
	for _, pod := range pods {
		podNames = append(podNames, pod.Name)
	}
	return podNames
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
		"productName":    "IBM Cloud Platform Common Services",
		"productID":      "068a62892a1e4db39641342e592daa25",
		"productVersion": "3.3.0",
		"productMetric":  "FREE",
	}
}
