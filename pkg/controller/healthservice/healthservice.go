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
	"io/ioutil"
	"os"
	"reflect"

	operatorv1alpha1 "github.com/IBM/ibm-healthcheck-operator/pkg/apis/operator/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/yaml"
)

var gracePeriod = int64(60)
var mode484 = int32(484)

func (r *ReconcileHealthService) createOrUpdateHealthServiceDeploy(h *operatorv1alpha1.HealthService) error {
	hsName := h.Spec.HealthService.Name
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
	}

	if err := r.updateHealthServiceDeployment(current, desired); err != nil {
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
	podNames := getPodNames(podList.Items)

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

func (r *ReconcileHealthService) createOrUpdateHealthServiceSvc(h *operatorv1alpha1.HealthService) error {
	hsName := h.Spec.HealthService.Name
	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)

	// Check if the service already exists, if not create a new one
	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: hsName, Namespace: h.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new service
		svc := r.desiredHealthServiceService(h)
		reqLogger.Info("Creating a new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		if err := r.client.Create(context.TODO(), svc); err != nil {
			reqLogger.Error(err, "Failed to create new Service", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Service", "Service.Namespace", found.Namespace, "Service.Name", found.Name)
		return err
	}

	return nil
}

func (r *ReconcileHealthService) createOrUpdateHealthServiceIngress(h *operatorv1alpha1.HealthService) error {
	hsName := h.Spec.HealthService.Name
	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)

	// Check if the ingress already exists, if not create a new one
	found := &extensionsv1.Ingress{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: hsName, Namespace: h.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		// Define a new ingress
		ing := r.desiredHealthServiceIngress(h)
		reqLogger.Info("Creating a new Ingress", "Ingress.Namespace", ing.Namespace, "Ingress.Name", ing.Name)
		if err := r.client.Create(context.TODO(), ing); err != nil {
			reqLogger.Error(err, "Failed to create new Ingress", "Ingress.Namespace", ing.Namespace, "Ingress.Name", ing.Name)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Ingress", "Ingress.Namespace", found.Namespace, "Ingress.Name", found.Name)
		return err
	}

	return nil
}

func (r *ReconcileHealthService) createOrUpdateHealthServiceConfigmap(h *operatorv1alpha1.HealthService) error {
	hsName := h.Spec.HealthService.Name
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

func (r *ReconcileHealthService) updateHealthServiceDeployment(current, desired *appsv1.Deployment) error {
	reqLogger := log.WithValues("Deployment.Namespace", current.Namespace, "Deployment.Name", current.Name)

	if current.Spec.Replicas != nil && *current.Spec.Replicas == *desired.Spec.Replicas {
		return nil
	}

	reqLogger.Info("Updating Deployment", "Deployment.Namespace", desired.Namespace, "Deployment.Name", desired.Name)
	if err := r.client.Update(context.TODO(), desired); err != nil {
		reqLogger.Error(err, "Failed to update Deployment", "Deployment.Namespace", desired.Namespace, "Deployment.Name", desired.Name)
		return err
	}

	return nil
}

func (r *ReconcileHealthService) desiredHealthServiceDeployment(h *operatorv1alpha1.HealthService) *appsv1.Deployment {
	hsName := h.Spec.HealthService.Name
	cfgName := h.Spec.HealthService.ConfigmapName
	labels := labelsForHealthService(hsName, h.Name)
	annotations := annotationsForHealthService()
	serviceAccountName := "default"
	if len(h.Spec.Memcached.ServiceAccountName) > 0 {
		serviceAccountName = h.Spec.Memcached.ServiceAccountName
	}

	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)
	reqLogger.Info("Building HealthService Deployment", "Deployment.Namespace", h.Namespace, "Deployment.Name", hsName)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hsName,
			Namespace: h.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			MinReadySeconds: 0,
			Replicas:        &h.Spec.HealthService.ReplicaCount,
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
					HostNetwork:                   h.Spec.HealthService.HostNetwork,
					HostPID:                       false,
					HostIPC:                       false,
					ServiceAccountName:            serviceAccountName,
					Containers: []corev1.Container{
						{
							Name:            hsName,
							Image:           h.Spec.HealthService.Image.Repository + ":" + h.Spec.HealthService.Image.Tag,
							ImagePullPolicy: corev1.PullPolicy(h.Spec.HealthService.Image.PullPolicy),
							SecurityContext: &h.Spec.HealthService.SecurityContext,
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
							Resources: corev1.ResourceRequirements{
								Limits: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    *cpu500,
									corev1.ResourceMemory: *memory128,
								},
								Requests: map[corev1.ResourceName]resource.Quantity{
									corev1.ResourceCPU:    *cpu50,
									corev1.ResourceMemory: *memory64,
								},
							},
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
										Port:   intstr.IntOrString{Type: intstr.Int, IntVal: 6967},
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
										Port:   intstr.IntOrString{Type: intstr.Int, IntVal: 6967},
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
					NodeSelector: h.Spec.HealthService.NodeSelector,
					Tolerations:  h.Spec.HealthService.Tolerations,
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

func (r *ReconcileHealthService) desiredHealthServiceService(h *operatorv1alpha1.HealthService) *corev1.Service {
	hsName := h.Spec.HealthService.Name
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
					Port:       6967,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: 6967},
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

func (r *ReconcileHealthService) desiredHealthServiceIngress(h *operatorv1alpha1.HealthService) *extensionsv1.Ingress {
	hsName := h.Spec.HealthService.Name
	labels := labelsForHealthService(hsName, h.Name)
	annotations := annotationsForHealthServiceIngress()

	reqLogger := log.WithValues("HealthService.Namespace", h.Namespace, "HealthService.Name", h.Name)
	reqLogger.Info("Building HealthService Ingress", "Ingress.Namespace", h.Namespace, "Ingress.Name", hsName)

	ing := &extensionsv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        hsName,
			Namespace:   h.Namespace,
			Labels:      labels,
			Annotations: annotations,
		},
		Spec: extensionsv1.IngressSpec{
			Rules: []extensionsv1.IngressRule{
				{
					IngressRuleValue: extensionsv1.IngressRuleValue{
						HTTP: &extensionsv1.HTTPIngressRuleValue{
							Paths: []extensionsv1.HTTPIngressPath{
								{
									Path: "/cluster-health/",
									Backend: extensionsv1.IngressBackend{
										ServiceName: hsName,
										ServicePort: intstr.IntOrString{Type: intstr.Int, IntVal: 6967}},
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
		"productName":    "IBM Cloud Platform Common Services",
		"productID":      "068a62892a1e4db39641342e592daa25",
		"productVersion": "3.3.0",
		"productMetric":  "FREE",
	}
}
