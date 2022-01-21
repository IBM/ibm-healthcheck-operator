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
	"io/ioutil"
	"os"
	"reflect"

	operatorv1alpha1 "github.com/IBM/ibm-healthcheck-operator/pkg/apis/operator/v1alpha1"
	common "github.com/IBM/ibm-healthcheck-operator/pkg/controller/common"
	constant "github.com/IBM/ibm-healthcheck-operator/pkg/controller/constant"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

var gracePeriod = int64(60)
var mode484 = int32(484)

var healthResourceName = "system-healthcheck-service"

func (r *ReconcileHealthService) createOrUpdateHealthServiceDeploy(h *operatorv1alpha1.HealthService) error {
	hsName := healthResourceName
	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)

	// Define a new deployment
	desired := r.desiredHealthServiceDeployment(h)
	// Check if the deployment already exists, if not create a new one
	current := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: hsName, Namespace: h.Namespace}, current)

	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", desired.Namespace, "Deployment.Name", desired.Name)
		if err := r.client.Create(context.TODO(), desired); err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", desired.Namespace, "Deployment.Name", desired.Name)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment", "Deployment.Namespace", current.Namespace, "Deployment.Name", current.Name)
		return err
	} else if err := r.updateHealthServiceDeployment(h, current, desired); err != nil {
		return err
	}

	// Update the HealthService status with the pod names
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(h.Namespace),
		client.MatchingLabels(labelsForHealthService(hsName, h.Name)),
	}
	if err = r.client.List(context.TODO(), podList, listOpts...); err != nil {
		reqLogger.Error(err, "Failed to list pods", "h.Namespace", h.Namespace, "h.Name", hsName)
		return err
	}
	podNames := common.GetPodNames(podList.Items)

	// Update status.HealthCheckNodes if needed
	if !reflect.DeepEqual(podNames, h.Status.HealthCheckNodes) {
		h.Status.HealthCheckNodes = podNames
		err := r.client.Status().Update(context.TODO(), h)
		if err != nil {
			reqLogger.Error(err, "Failed to update HealthService status")
			return err
		}
	}

	return nil
}

func (r *ReconcileHealthService) createOrUpdateHealthServiceService(h *operatorv1alpha1.HealthService) error {
	hsName := healthResourceName
	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)

	// Define a new service
	desired := r.desiredHealthServiceService(h)
	// Check if the service already exists, if not create a new one
	current := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: hsName, Namespace: h.Namespace}, current)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Service", "Service.Namespace", desired.Namespace, "Service.Name", desired.Name)
		if err := r.client.Create(context.TODO(), desired); err != nil {
			reqLogger.Error(err, "Failed to create new Service", "Service.Namespace", desired.Namespace, "Service.Name", desired.Name)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Service", "Service.Namespace", current.Namespace, "Service.Name", current.Name)
		return err
	} else if err := r.updateHealthServiceService(h, current, desired); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileHealthService) createOrUpdateHealthServiceIngress(h *operatorv1alpha1.HealthService) error {
	hsName := healthResourceName
	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)

	// Define a new ingress
	desired := r.desiredHealthServiceIngress(h)
	// Check if the ingress already exists, if not create a new one
	current := &networkingv1.Ingress{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: hsName, Namespace: h.Namespace}, current)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Ingress", "Ingress.Namespace", desired.Namespace, "Ingress.Name", desired.Name)
		if err := r.client.Create(context.TODO(), desired); err != nil {
			reqLogger.Error(err, "Failed to create new Ingress", "Ingress.Namespace", desired.Namespace, "Ingress.Name", desired.Name)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Ingress", "Ingress.Namespace", current.Namespace, "Ingress.Name", current.Name)
		return err
	} else if err := r.updateHealthServiceIngress(h, current, desired); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileHealthService) createOrUpdateHealthServiceConfigmap(h *operatorv1alpha1.HealthService) error {
	hsName := healthResourceName
	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)
	labels := labelsForHealthService(hsName, h.Name)

	//read configmap from yaml
	yamlFile, err := os.Open("/manifests/system-healthcheck-service-config.yaml")
	if err != nil {
		reqLogger.Error(err, "Error opening System config file")
	}
	// defer the closing of our jsonFile so that we can parse it later on
	defer yamlFile.Close()
	byteValue, _ := ioutil.ReadAll(yamlFile)

	cm := new(corev1.ConfigMap)
	if err := yaml.Unmarshal(byteValue, cm); err != nil {
		reqLogger.Error(err, "Error parsing the configmap value from /manifests/system-healthcheck-service-config.yaml")
		return err
	}
	yamlFile.Close()

	//setup configmap name, namespace and labels
	cm.ObjectMeta.Name = h.Spec.HealthService.ConfigmapName
	cm.ObjectMeta.Namespace = h.Namespace
	cm.ObjectMeta.Labels = labels

	// Set HealthService instance as the owner and controller
	if err := controllerutil.SetControllerReference(h, cm, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "configmap.Namespace", cm.Namespace, "configmap.Name", cm.Name)
	}

	// Check if the ingress already exists, if not create a new one
	found := &corev1.ConfigMap{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: cm.Name, Namespace: cm.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new configmap
		reqLogger.Info("Creating a new configmap", "configmap.Namespace", cm.Namespace, "configmap.Name", cm.Name)
		if err := r.client.Create(context.TODO(), cm); err != nil {
			reqLogger.Error(err, "Failed to create new configmap", "configmap.Namespace", cm.Namespace, "configmap.Name", cm.Name)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Ingress", "configmap.Namespace", found.Namespace, "configmap.Name", found.Name)
		return err
	}

	return nil
}

func (r *ReconcileHealthService) updateHealthServiceDeployment(h *operatorv1alpha1.HealthService, current, desired *appsv1.Deployment) error {
	reqLogger := log.WithValues("Deployment.Namespace", current.Namespace, "Deployment.Name", current.Name)

	updated := current.DeepCopy()
	updated.Spec.Replicas = desired.Spec.Replicas
	updated.Spec.Template.ObjectMeta.Annotations = desired.Spec.Template.ObjectMeta.Annotations
	updated.Spec.Template.Spec.Containers = desired.Spec.Template.Spec.Containers
	updated.Spec.Template.Spec.Volumes = desired.Spec.Template.Spec.Volumes

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

func (r *ReconcileHealthService) desiredHealthServiceDeployment(h *operatorv1alpha1.HealthService) *appsv1.Deployment {
	hsName := healthResourceName
	cfgName := h.Spec.HealthService.ConfigmapName
	labels := labelsForHealthService(hsName, h.Name)
	annotations := annotationsForHealthService()
	serviceAccountName := "ibm-healthcheck-operator-cluster"

	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)
	reqLogger.Info("Building HealthService Deployment", "Deployment.Namespace", h.Namespace, "Deployment.Name", hsName)

	hsResources := common.GetResources(&h.Spec.HealthService.Resources)
	hsReplicas := int32(1)
	if h.Spec.HealthService.Replicas > 0 {
		hsReplicas = h.Spec.HealthService.Replicas
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hsName,
			Namespace: h.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			MinReadySeconds: 0,
			// Replicas:        &h.Spec.HealthService.ReplicaCount,
			Replicas: &hsReplicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &gracePeriod,
					HostNetwork:                   false,
					HostPID:                       false,
					HostIPC:                       false,
					ServiceAccountName:            serviceAccountName,
					Containers: []corev1.Container{
						{
							Name:            hsName,
							Image:           os.Getenv("SYSTEM_HEALTHCHECK_SERVICE_IMAGE"),
							ImagePullPolicy: corev1.PullIfNotPresent,
							SecurityContext: &commonSecurityContext,
							Env: []corev1.EnvVar{
								{
									Name: "HEALTHNAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name:  "CPNAMESCONFIGPATH",
									Value: "/etc/health/cpnames.yaml",
								},
								{
									Name:  "LOGLEVEL",
									Value: "1",
								},
								{
									Name:  "MEMCACHEDPORT",
									Value: "11211",
								},
								{
									Name:  "CLOUDPAKNAME_SETTING",
									Value: h.Spec.HealthService.CloudpakNameSetting,
								},
								{
									Name:  "SERVICENAME_SETTING",
									Value: h.Spec.HealthService.ServiceNameSetting,
								},
								{
									Name:  "DEPENDS_SETTING",
									Value: h.Spec.HealthService.DependsSetting,
								},
							},
							Resources: *hsResources,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "cluster-healthcheck-data",
									MountPath: "/etc/health",
								},
								{
									Name:      "tmp-volume",
									MountPath: "/tmp",
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Port:   intstr.IntOrString{Type: intstr.Int, IntVal: constant.HealthServicePort},
										Path:   "/v1alpha1/health",
										Scheme: "HTTP",
									},
								},
								FailureThreshold:    3,
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								TimeoutSeconds:      2,
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Port:   intstr.IntOrString{Type: intstr.Int, IntVal: constant.HealthServicePort},
										Path:   "/v1alpha1/health",
										Scheme: "HTTP",
									},
								},
								FailureThreshold:    1,
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								SuccessThreshold:    1,
								TimeoutSeconds:      2,
							},
						},
					},
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
					Volumes: []corev1.Volume{
						{
							Name: "tmp-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "cluster-healthcheck-data",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: cfgName,
									},
									DefaultMode: &mode484,
									Items: []corev1.KeyToPath{
										{
											Key:  "cpnames.yaml",
											Path: "cpnames.yaml",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Set HealthService instance as the owner and controller
	if err := controllerutil.SetControllerReference(h, dep, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Deployment.Namespace", h.Namespace, "Deployment.Name", hsName)
	}

	return dep
}

func (r *ReconcileHealthService) updateHealthServiceService(h *operatorv1alpha1.HealthService, current, desired *corev1.Service) error {
	reqLogger := log.WithValues("Service.Namespace", current.Namespace, "Service.Name", current.Name)

	updated := current.DeepCopy()
	updated.ObjectMeta.Labels = desired.ObjectMeta.Labels
	updated.Spec.Ports = desired.Spec.Ports
	updated.Spec.Selector = desired.Spec.Selector
	updated.Spec.Type = desired.Spec.Type

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

func (r *ReconcileHealthService) desiredHealthServiceService(h *operatorv1alpha1.HealthService) *corev1.Service {
	hsName := healthResourceName
	labels := labelsForHealthService(hsName, h.Name)

	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)
	reqLogger.Info("Building HealthService Service", "Service.Namespace", h.Namespace, "Service.Name", hsName)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            hsName,
			Namespace:       h.Namespace,
			Labels:          labels,
			ResourceVersion: "",
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       constant.HealthServicePort,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: constant.HealthServicePort},
				},
			},
			Selector: labels,
			Type:     corev1.ServiceTypeClusterIP,
		},
	}

	// Set HealthService instance as the owner and controller
	if err := controllerutil.SetControllerReference(h, svc, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Service.Namespace", h.Namespace, "Service.Name", hsName)
	}

	return svc
}

func (r *ReconcileHealthService) updateHealthServiceIngress(h *operatorv1alpha1.HealthService, current, desired *networkingv1.Ingress) error {
	reqLogger := log.WithValues("Ingress.Namespace", current.Namespace, "Ingress.Name", current.Name)

	updated := current.DeepCopy()
	updated.ObjectMeta.Labels = desired.ObjectMeta.Labels
	updated.ObjectMeta.Annotations = desired.ObjectMeta.Annotations
	updated.Spec.Rules = desired.Spec.Rules

	reqLogger.Info("Updating Ingress")
	// Set HealthService instance as the owner and controller
	if err := controllerutil.SetControllerReference(h, updated, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Ingress.Namespace", updated.Namespace, "Ingress.Name", updated.Name)
	}

	if err := r.client.Update(context.TODO(), updated); err != nil {
		reqLogger.Error(err, "Failed to update Ingress", "Ingress.Namespace", updated.Namespace, "Ingress.Name", updated.Name)
		return err
	}

	return nil

}

func (r *ReconcileHealthService) desiredHealthServiceIngress(h *operatorv1alpha1.HealthService) *networkingv1.Ingress {
	hsName := healthResourceName
	labels := labelsForHealthService(hsName, h.Name)
	annotations := annotationsForHealthServiceIngress()
	pathType := networkingv1.PathType(constant.IngPathType)

	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)
	reqLogger.Info("Building HealthService Ingress", "Ingress.Namespace", h.Namespace, "Ingress.Name", hsName)

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        hsName,
			Namespace:   h.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     constant.HealthServiceRoute,
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: hsName,
											Port: networkingv1.ServiceBackendPort{Number: constant.HealthServicePort},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// Set HealthService instance as the owner and controller
	if err := controllerutil.SetControllerReference(h, ing, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Ingress.Namespace", h.Namespace, "Ingress.Name", hsName)
	}

	return ing
}

func annotationsForHealthServiceIngress() map[string]string {
	return map[string]string{
		"kubernetes.io/ingress.class":           "ibm-icp-management",
		"icp.management.ibm.com/rewrite-target": "/",
		"icp.management.ibm.com/configuration-snippet": `add_header Cache-Control "no-cache, no-store, must-revalidate";
            add_header Pragma no-cache;
            add_header Expires 0;
            add_header X-Frame-Options "SAMEORIGIN";
            add_header X-Content-Type-Options nosniff;
            add_header X-XSS-Protection "1; mode=block";`,
	}
}

func labelsForHealthService(name string, releaseName string) map[string]string {
	return map[string]string{
		"app":                          name,
		"release":                      releaseName,
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/instance":   releaseName,
		"app.kubernetes.io/managed-by": "",
	}
}

func annotationsForHealthService() map[string]string {
	return map[string]string{
		"productName":   "IBM Cloud Platform Common Services",
		"productID":     "068a62892a1e4db39641342e592daa25",
		"productMetric": "FREE",
	}
}
