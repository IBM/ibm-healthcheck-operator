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
package mustgatherservice

import (
	"context"
	"io/ioutil"
	"os"
	"reflect"

	operatorv1alpha1 "github.com/IBM/ibm-healthcheck-operator/pkg/apis/operator/v1alpha1"

	common "github.com/IBM/ibm-healthcheck-operator/pkg/controller/common"
	constant "github.com/IBM/ibm-healthcheck-operator/pkg/controller/constant"

	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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

var trueVar = true
var falseVar = false

var mustGatherResourceName = "must-gather-service"

var mustGatherCustomCMName = "ibm-mustgather-customscript-default"

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

func (r *ReconcileMustGatherService) createOrUpdateMustGatherServiceStatefulSet(instance *operatorv1alpha1.MustGatherService) error {
	reqLogger := log.WithValues("MustGatherService.Namespace", instance.Namespace, "MustGatherService.Name", instance.Name)

	// Define a new StatefulSet
	desired := r.desiredMustGatherServiceStatefulset(instance)
	// Check if the StatefulSet already exists, if not create a new one
	current := &appsv1.StatefulSet{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: mustGatherResourceName, Namespace: instance.Namespace}, current)

	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new StatefulSet", "StatefulSet.Namespace", desired.Namespace, "StatefulSet.Name", desired.Name)
		if err := r.client.Create(context.TODO(), desired); err != nil {
			reqLogger.Error(err, "Failed to create new StatefulSet", "StatefulSet.Namespace", desired.Namespace, "StatefulSet.Name", desired.Name)
			return err
		}
	} else if err != nil {
		reqLogger.Error(err, "Failed to get StatefulSet", "StatefulSet.Namespace", current.Namespace, "StatefulSet.Name", current.Name)
		return err
	} else if err := r.updateMustGatherServiceStatefulSet(instance, current, desired); err != nil {
		return err
	}

	// Update the MustGatherService status with the pod names
	podList := &corev1.PodList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
		client.MatchingLabels(labelsForMustGatherService(mustGatherResourceName, instance.Name)),
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

func (r *ReconcileMustGatherService) updateMustGatherServiceStatefulSet(instance *operatorv1alpha1.MustGatherService,
	current, desired *appsv1.StatefulSet) error {
	reqLogger := log.WithValues("StatefulSet.Namespace", current.Namespace, "StatefulSet.Name", current.Name)

	updated := current.DeepCopy()
	updated.Spec.Replicas = desired.Spec.Replicas
	updated.Spec.Template.Spec.Containers = desired.Spec.Template.Spec.Containers
	updated.Spec.Template.Spec.Volumes = desired.Spec.Template.Spec.Volumes

	reqLogger.Info("Updating StatefulSet")
	// Set MustGatherService instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, updated, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "StatefulSet.Namespace", updated.Namespace, "StatefulSet.Name", updated.Name)
	}

	if err := r.client.Update(context.TODO(), updated); err != nil {
		reqLogger.Error(err, "Failed to update StatefulSet", "StatefulSet.Namespace", updated.Namespace, "StatefulSet.Name", updated.Name)
		return err
	}

	return nil
}

func (r *ReconcileMustGatherService) desiredMustGatherServiceStatefulset(instance *operatorv1alpha1.MustGatherService) *appsv1.StatefulSet {
	appName := "must-gather-service"
	labels := labelsForMustGatherService(appName, instance.Name)
	annotations := annotationsForMustGatherService()

	serviceAccountName := "ibm-healthcheck-operator"
	defaultCommand := []string{"/bin/must-gather-service", "-v", "1"}

	reqLogger := log.WithValues("MustGatherService.Namespace", instance.Namespace, "MustGatherService.Name", instance.Name)
	reqLogger.Info("Building MustGatherService StatefulSet", "StatefulSet.Namespace", instance.Namespace, "StatefulSet.Name", appName)

	appResources := common.GetResources(&instance.Spec.MustGather.Resources)
	appReplicas := int32(1)
	if instance.Spec.MustGather.Replicas > 0 {
		appReplicas = instance.Spec.MustGather.Replicas
	}

	dep := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: instance.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &appReplicas,
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
							Name:            appName,
							Image:           os.Getenv("MUST_GATHER_SERVICE_IMAGE"),
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         defaultCommand,
							SecurityContext: &commonSecurityContext,
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
										Port:   intstr.IntOrString{Type: intstr.Int, IntVal: constant.MustgatherServicePort},
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
										Port:   intstr.IntOrString{Type: intstr.Int, IntVal: constant.MustgatherServicePort},
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
		reqLogger.Error(err, "SetControllerReference failed", "StatefulSet.Namespace", instance.Namespace, "StatefulSet.Name", appName)
	}

	return dep
}

func (r *ReconcileMustGatherService) createOrUpdateMustGatherServiceService(instance *operatorv1alpha1.MustGatherService) error {
	appName := mustGatherResourceName
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
	appName := mustGatherResourceName
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
					Port:       constant.MustgatherServicePort,
					TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: constant.MustgatherServicePort},
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

func (r *ReconcileMustGatherService) createOrUpdateMustGatherServiceConfigmap(instance *operatorv1alpha1.MustGatherService) error {
	appName := mustGatherResourceName
	reqLogger := log.WithValues("MustGatherService.Namespace", instance.Namespace, "MustGatherService.Name", instance.Name)
	labels := labelsForMustGatherServiceCustomCM(appName, instance.Name)

	//read configmap from yaml
	yamlFile, err := os.Open("/manifests/ibm-mustgather-customscript-default.yaml")
	if err != nil {
		reqLogger.Error(err, "Error opening mustgather custom config file")
	}
	// defer the closing of our jsonFile so that we can parse it later on
	defer yamlFile.Close()
	byteValue, _ := ioutil.ReadAll(yamlFile)

	cm := new(corev1.ConfigMap)
	if err := yaml.Unmarshal(byteValue, cm); err != nil {
		reqLogger.Error(err, "Error parsing the configmap value from /manifests/ibm-mustgather-customscript-default.yaml")
		return err
	}
	yamlFile.Close()

	//setup configmap name, namespace and labels
	cm.ObjectMeta.Name = mustGatherCustomCMName
	cm.ObjectMeta.Namespace = instance.Namespace
	cm.ObjectMeta.Labels = labels

	// Set mustgatherservice instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, cm, r.scheme); err != nil {
		reqLogger.Error(err, "SetControllerReference failed", "configmap.Namespace", cm.Namespace, "configmap.Name", cm.Name)
	}

	// Check if the configmap already exists, if not create a new one
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
		reqLogger.Error(err, "Failed to get configmap", "configmap.Namespace", found.Namespace, "configmap.Name", found.Name)
		return err
	}

	return nil
}

func (r *ReconcileMustGatherService) createOrUpdateMustGatherServiceIngress(instance *operatorv1alpha1.MustGatherService) error {
	appName := mustGatherResourceName
	reqLogger := log.WithValues("MustGatherService.Namespace", instance.Namespace, "MustGatherService.Name", instance.Name)

	// Define a new ingress
	desired := r.desiredMustGatherServiceIngress(instance)
	// Check if the ingress already exists, if not create a new one
	current := &networkingv1.Ingress{}
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
	current, desired *networkingv1.Ingress) error {
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

func (r *ReconcileMustGatherService) desiredMustGatherServiceIngress(instance *operatorv1alpha1.MustGatherService) *networkingv1.Ingress {
	appName := mustGatherResourceName
	labels := labelsForMustGatherService(appName, instance.Name)
	annotations := annotationsForMustGatherServiceIngress()
	pathType := networkingv1.PathType(constant.IngPathType)

	reqLogger := log.WithValues("MustGatherService.Namespace", instance.Namespace, "MustGatherService.Name", instance.Name)
	reqLogger.Info("Building MustGatherService Ingress", "Ingress.Namespace", instance.Namespace, "Ingress.Name", appName)

	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        appName,
			Namespace:   instance.Namespace,
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
									Path:     constant.MustgatherServiceRoute,
									PathType: &pathType,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: appName,
											Port: networkingv1.ServiceBackendPort{Number: constant.MustgatherServicePort},
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
		"app.kubernetes.io/managed-by": "",
	}
}

func labelsForMustGatherServiceCustomCM(name string, releaseName string) map[string]string {
	return map[string]string{
		"app":                          name,
		"release":                      releaseName,
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/instance":   releaseName,
		"app.kubernetes.io/managed-by": "",
		"serviceability-addon":         "default",
	}
}

func annotationsForMustGatherService() map[string]string {
	return map[string]string{
		"productName":   "IBM Cloud Platform Common Services",
		"productID":     "068a62892a1e4db39641342e592daa25",
		"productMetric": "FREE",
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
