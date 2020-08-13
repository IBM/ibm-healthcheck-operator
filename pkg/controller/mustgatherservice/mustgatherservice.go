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
package mustgatherservice

import (
	"context"
	"os"
	"reflect"

	operatorv1alpha1 "github.com/IBM/ibm-healthcheck-operator/pkg/apis/operator/v1alpha1"

	common "github.com/IBM/ibm-healthcheck-operator/pkg/controller/common"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	extensionsv1 "k8s.io/api/extensions/v1beta1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var gracePeriod = int64(60)

func (r *ReconcileMustGatherService) createOrUpdateMustGatherServiceDeploy(instance *operatorv1alpha1.MustGatherService) error {
	reqLogger := log.WithValues("MustGatherService.Namespace", instance.Namespace, "MustGatherService.Name", instance.Name)

	// Define a new deployment
	desired := r.desiredMustGatherServiceDeployment(instance)
	// Check if the deployment already exists, if not create a new one
	current := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: instance.Spec.MustGather.Name, Namespace: instance.Namespace}, current)

	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Deployment", "Deployment.Namespace", desired.Namespace, "Deployment.Name", desired.Name)
		if err := r.client.Create(context.TODO(), desired); err != nil {
			reqLogger.Error(err, "Failed to create new Deployment", "Deployment.Namespace", desired.Namespace, "Deployment.Name", desired.Name)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Deployment", "Deployment.Namespace", current.Namespace, "Deployment.Name", current.Name)
		return err
	} else if err := r.updateMustGatherServiceDeployment(instance, current, desired); err != nil {
		return err
	}

	// Update the MustGatherService status with the pod names
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.MatchingLabels(labelsForMustGatherService(instance.Spec.MustGather.Name, instance.Name)),
	}
	if err = r.client.List(context.TODO(), podList, listOpts...); err != nil {
		reqLogger.Error(err, "Failed to list pods", "instance.Namespace", instance.Namespace, "instance.Name", instance.Name)
		return err
	}
	podNames := common.GetPodNames(podList.Items)

	// Update status.MustGatherServiceNodes if needed
	if !reflect.DeepEqual(podNames, instance.Status.MustGatherServiceNodes) {
		instance.Status.MustGatherServiceNodes = podNames
		err := r.client.Status().Update(context.TODO(), instance)
		if err != nil {
			reqLogger.Error(err, "Failed to update MustGatherService status")
			return err
		}
	}

	return nil
}

func (r *ReconcileMustGatherService) updateMustGatherServiceDeployment(instance *operatorv1alpha1.MustGatherService,
	current, desired *appsv1.Deployment) error {
	reqLogger := log.WithValues("Deployment.Namespace", current.Namespace, "Deployment.Name", current.Name)

	updated := current.DeepCopy()
	updated.ObjectMeta.Labels = desired.ObjectMeta.Labels
	updated.Spec.MinReadySeconds = desired.Spec.MinReadySeconds
	updated.Spec.Replicas = desired.Spec.Replicas
	updated.Spec.Selector.MatchLabels = desired.Spec.Selector.MatchLabels
	updated.Spec.Template.ObjectMeta.Labels = desired.Spec.Template.ObjectMeta.Labels
	updated.Spec.Template.ObjectMeta.Annotations = desired.Spec.Template.ObjectMeta.Annotations
	updated.Spec.Template.Spec.TerminationGracePeriodSeconds = desired.Spec.Template.Spec.TerminationGracePeriodSeconds
	updated.Spec.Template.Spec.HostNetwork = desired.Spec.Template.Spec.HostNetwork
	updated.Spec.Template.Spec.HostPID = desired.Spec.Template.Spec.HostPID
	updated.Spec.Template.Spec.HostIPC = desired.Spec.Template.Spec.HostIPC
	updated.Spec.Template.Spec.ServiceAccountName = desired.Spec.Template.Spec.ServiceAccountName
	updated.Spec.Template.Spec.Containers = desired.Spec.Template.Spec.Containers
	updated.Spec.Template.Spec.NodeSelector = desired.Spec.Template.Spec.NodeSelector
	updated.Spec.Template.Spec.Tolerations = desired.Spec.Template.Spec.Tolerations
	updated.Spec.Template.Spec.Volumes = desired.Spec.Template.Spec.Volumes

	reqLogger.Info("Updating Deployment")
	// Set MustGatherService instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, updated, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Deployment.Namespace", updated.Namespace, "Deployment.Name", updated.Name)
	}

	if err := r.client.Update(context.TODO(), updated); err != nil {
		reqLogger.Error(err, "Failed to update Deployment", "Deployment.Namespace", updated.Namespace, "Deployment.Name", updated.Name)
		return err
	}

	return nil
}

func (r *ReconcileMustGatherService) desiredMustGatherServiceDeployment(instance *operatorv1alpha1.MustGatherService) *appsv1.Deployment {
	appName := instance.Spec.MustGather.Name
	labels := labelsForMustGatherService(appName, instance.Name)
	annotations := annotationsForMustGatherService()
	serviceAccountName := "default"
	if len(instance.Spec.MustGather.ServiceAccountName) > 0 {
		serviceAccountName = instance.Spec.MustGather.ServiceAccountName
	}
	defaultCommand := []string{"/bin/must-gather-service", "-v", "1"}
	if instance.Spec.MustGather.Command != nil && len(instance.Spec.MustGather.Command) > 0 {
		defaultCommand = instance.Spec.MustGather.Command
	}

	reqLogger := log.WithValues("MustGatherService.Namespace", instance.Namespace, "MustGatherService.Name", instance.Name)
	reqLogger.Info("Building MustGatherService Deployment", "Deployment.Namespace", instance.Namespace, "Deployment.Name", appName)

	appResources := common.GetResources(&instance.Spec.MustGather.Resources)
	appReplicas := int32(1)
	if instance.Spec.MustGather.Replicas > 0 {
		appReplicas = instance.Spec.MustGather.Replicas
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			MinReadySeconds: 0,
			Replicas:        &appReplicas,
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
					HostNetwork:                   instance.Spec.MustGather.HostNetwork,
					HostPID:                       false,
					HostIPC:                       false,
					ServiceAccountName:            serviceAccountName,
					Containers: []corev1.Container{
						{
							Name:            appName,
							Image:           os.Getenv("OPERAND_MUSTGATHER_SERVICE_IMAGE"),
							ImagePullPolicy: corev1.PullPolicy(instance.Spec.MustGather.Image.PullPolicy),
							Command:         defaultCommand,
							SecurityContext: &instance.Spec.MustGather.SecurityContext,
							Env: []corev1.EnvVar{
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name:  "LOGLEVEL",
									Value: "1",
								},
							},
							Resources: *appResources,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "must-gather",
									MountPath: "/must-gather",
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Port:   intstr.IntOrString{Type: intstr.Int, IntVal: 6967},
										Path:   "/v1alpha1/healthz",
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
										Path:   "/v1alpha1/healthz",
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
					NodeSelector: instance.Spec.MustGather.NodeSelector,
					Tolerations:  instance.Spec.MustGather.Tolerations,
					Volumes: []corev1.Volume{
						{
							Name: "must-gather",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: instance.Spec.PersistentVolumeClaim.Name,
								},
							},
						},
					},
				},
			},
		},
	}

	// Set MustGatherService instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, dep, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Deployment.Namespace", instance.Namespace, "Deployment.Name", appName)
	}

	return dep
}

func (r *ReconcileMustGatherService) createOrUpdateMustGatherServiceService(instance *operatorv1alpha1.MustGatherService) error {
	appName := instance.Spec.MustGather.Name
	reqLogger := log.WithValues("MustGatherService.Namespace", instance.Namespace, "MustGatherService.Name", instance.Name)

	// Define a new service
	desired := r.desiredMustGatherServiceService(instance)
	// Check if the service already exists, if not create a new one
	current := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: appName, Namespace: instance.Namespace}, current)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Service", "Service.Namespace", desired.Namespace, "Service.Name", desired.Name)
		if err := r.client.Create(context.TODO(), desired); err != nil {
			reqLogger.Error(err, "Failed to create new Service", "Service.Namespace", desired.Namespace, "Service.Name", desired.Name)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Service", "Service.Namespace", current.Namespace, "Service.Name", current.Name)
		return err
	} else if err := r.updateMustGatherServiceService(instance, current, desired); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileMustGatherService) updateMustGatherServiceService(instance *operatorv1alpha1.MustGatherService, current, desired *corev1.Service) error {
	reqLogger := log.WithValues("Service.Namespace", current.Namespace, "Service.Name", current.Name)

	updated := current.DeepCopy()
	updated.ObjectMeta.Labels = desired.ObjectMeta.Labels
	updated.Spec.Ports = desired.Spec.Ports
	updated.Spec.Selector = desired.Spec.Selector
	updated.Spec.Type = desired.Spec.Type

	reqLogger.Info("Updating Service")
	// Set MustGatherService instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, updated, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Service.Namespace", updated.Namespace, "Service.Name", updated.Name)
	}

	if err := r.client.Update(context.TODO(), updated); err != nil {
		reqLogger.Error(err, "Failed to update Service", "Service.Namespace", updated.Namespace, "Service.Name", updated.Name)
		return err
	}

	return nil
}

func (r *ReconcileMustGatherService) desiredMustGatherServiceService(instance *operatorv1alpha1.MustGatherService) *corev1.Service {
	appName := instance.Spec.MustGather.Name
	labels := labelsForMustGatherService(appName, instance.Name)

	reqLogger := log.WithValues("MustGatherService.Namespace", instance.Namespace, "MustGatherService.Name", instance.Name)
	reqLogger.Info("Building MustGatherService Service", "Service.Namespace", instance.Namespace, "Service.Name", appName)

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:            appName,
			Namespace:       instance.Namespace,
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

	// Set MustGatherService instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, svc, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Service.Namespace", instance.Namespace, "Service.Name", appName)
	}

	return svc
}

func (r *ReconcileMustGatherService) createOrUpdateMustGatherServiceIngress(instance *operatorv1alpha1.MustGatherService) error {
	appName := instance.Spec.MustGather.Name
	reqLogger := log.WithValues("MustGatherService.Namespace", instance.Namespace, "MustGatherService.Name", instance.Name)

	// Define a new ingress
	desired := r.desiredMustGatherServiceIngress(instance)
	// Check if the ingress already exists, if not create a new one
	current := &extensionsv1.Ingress{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: appName, Namespace: instance.Namespace}, current)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Ingress", "Ingress.Namespace", desired.Namespace, "Ingress.Name", desired.Name)
		if err := r.client.Create(context.TODO(), desired); err != nil {
			reqLogger.Error(err, "Failed to create new Ingress", "Ingress.Namespace", desired.Namespace, "Ingress.Name", desired.Name)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get Ingress", "Ingress.Namespace", current.Namespace, "Ingress.Name", current.Name)
		return err
	} else if err := r.updateMustGatherServiceIngress(instance, current, desired); err != nil {
		return err
	}

	return nil
}

func (r *ReconcileMustGatherService) updateMustGatherServiceIngress(instance *operatorv1alpha1.MustGatherService,
	current, desired *extensionsv1.Ingress) error {
	reqLogger := log.WithValues("Ingress.Namespace", current.Namespace, "Ingress.Name", current.Name)

	updated := current.DeepCopy()
	updated.ObjectMeta.Labels = desired.ObjectMeta.Labels
	updated.ObjectMeta.Annotations = desired.ObjectMeta.Annotations
	updated.Spec.Rules = desired.Spec.Rules

	reqLogger.Info("Updating Ingress")
	// Set MustGatherService instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, updated, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Ingress.Namespace", updated.Namespace, "Ingress.Name", updated.Name)
	}

	if err := r.client.Update(context.TODO(), updated); err != nil {
		reqLogger.Error(err, "Failed to update Ingress", "Ingress.Namespace", updated.Namespace, "Ingress.Name", updated.Name)
		return err
	}

	return nil

}

func (r *ReconcileMustGatherService) desiredMustGatherServiceIngress(instance *operatorv1alpha1.MustGatherService) *extensionsv1.Ingress {
	appName := instance.Spec.MustGather.Name
	labels := labelsForMustGatherService(appName, instance.Name)
	annotations := annotationsForMustGatherServiceIngress()

	reqLogger := log.WithValues("MustGatherService.Namespace", instance.Namespace, "MustGatherService.Name", instance.Name)
	reqLogger.Info("Building MustGatherService Ingress", "Ingress.Namespace", instance.Namespace, "Ingress.Name", appName)

	ing := &extensionsv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        appName,
			Namespace:   instance.Namespace,
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
									Path: "/must-gather/",
									Backend: extensionsv1.IngressBackend{
										ServiceName: appName,
										ServicePort: intstr.IntOrString{Type: intstr.Int, IntVal: 6967}},
								},
							},
						},
					},
				},
			},
		},
	}

	// Set MustGatherService instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, ing, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "Ingress.Namespace", instance.Namespace, "Ingress.Name", appName)
	}

	return ing
}

func (r *ReconcileMustGatherService) createOrUpdateMustGatherServicePVC(instance *operatorv1alpha1.MustGatherService) error {
	pvcName := instance.Spec.PersistentVolumeClaim.Name
	reqLogger := log.WithValues("MustGatherService.Namespace", instance.Namespace, "MustGatherService.Name", instance.Name)

	// Define must gather persistence storage
	desired := r.desiredMustGatherServicePVC(instance)

	// Check if this pvc already exists
	current := &corev1.PersistentVolumeClaim{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: pvcName, Namespace: instance.Namespace}, current)

	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new pvc", "pvc.Namespace", desired.Namespace, "pvc.Name", desired.Name)
		err = r.client.Create(context.TODO(), desired)
		if err != nil {
			return err
		}
		// pvc created successfully - don't requeue
		return nil
	} else if err != nil {
		reqLogger.Error(err, "Failed to get pvc", "pvc.Namespace", current.Namespace, "pvc.Name", current.Name)
		return err
	} else {
		// pvc already exists - don't requeue
		reqLogger.Info("Skip reconcile: pvc already exists", "pvc.Namespace", current.Namespace, "pvc.Name", current.Name)
		return nil
	}
}

// newMustGatherPVC create a pvc for must gather service
func (r *ReconcileMustGatherService) desiredMustGatherServicePVC(instance *operatorv1alpha1.MustGatherService) *corev1.PersistentVolumeClaim {
	var storageClassName string
	var storageRequest resource.Quantity

	reqLogger := log.WithValues("MustGatherService.Namespace", instance.Namespace, "MustGatherService.Name", instance.Name)
	reqLogger.Info("Building MustGatherService PVC", "PVC.Namespace", instance.Namespace, "PVC.Name", instance.Spec.PersistentVolumeClaim.Name)

	if instance.Spec.PersistentVolumeClaim.StorageClassName != "" {
		storageClassName = instance.Spec.PersistentVolumeClaim.StorageClassName
	} else {
		storageClassName = r.getDefaultStorageClass()
	}

	if val, ok := instance.Spec.PersistentVolumeClaim.Resources.Requests[v1.ResourceStorage]; ok {
		storageRequest = val
	} else {
		storageRequest = resource.MustParse("2Gi")
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:        instance.Spec.PersistentVolumeClaim.Name,
			Namespace:   instance.Namespace,
			Labels:      labelsForMustGatherService(instance.Spec.PersistentVolumeClaim.Name, instance.Name),
			Annotations: annotationsForMustGatherService(),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: storageRequest,
				},
			},
			StorageClassName: &storageClassName,
		},
	}
	// Set MustGatherService instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pvc, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "pvc.Namespace", instance.Namespace, "pvc.Name", instance.Spec.PersistentVolumeClaim.Name)
	}

	return pvc
}

func (r *ReconcileMustGatherService) getDefaultStorageClass() string {
	scList := &storagev1.StorageClassList{}
	err := r.reader.List(context.TODO(), scList)
	if err != nil {
		return ""
	}
	if len(scList.Items) == 0 {
		return ""
	}

	var defaultSC []string
	var nonDefaultSC []string

	for _, sc := range scList.Items {
		if sc.Provisioner == "kubernetes.io/no-provisioner" {
			continue
		}
		if sc.ObjectMeta.GetAnnotations()["storageclass.kubernetes.io/is-default-class"] == "true" {
			defaultSC = append(defaultSC, sc.GetName())
			continue
		}
		nonDefaultSC = append(nonDefaultSC, sc.GetName())
	}

	if len(defaultSC) != 0 {
		return defaultSC[0]
	}

	if len(nonDefaultSC) != 0 {
		return nonDefaultSC[0]
	}

	return ""
}

func labelsForMustGatherService(name string, releaseName string) map[string]string {
	return map[string]string{
		"app":                          name,
		"release":                      releaseName,
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/instance":   releaseName,
		"app.kubernetes.io/managed-by": "ibm-healthcheck-operator",
	}
}

func annotationsForMustGatherService() map[string]string {
	return map[string]string{
		"productName":    "IBM Cloud Platform Common Services",
		"productID":      "068a62892a1e4db39641342e592daa25",
		"productVersion": "3.4.0",
		"productMetric":  "FREE",
	}
}

func annotationsForMustGatherServiceIngress() map[string]string {
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
