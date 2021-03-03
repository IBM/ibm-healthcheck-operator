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

package mustgatherjob

import (
	"context"
	"os"
	"strings"

	operatorv1alpha1 "github.com/IBM/ibm-healthcheck-operator/pkg/apis/operator/v1alpha1"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_mustgatherjob")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new MustGatherJob Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileMustGatherJob{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("mustgatherjob-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource MustGatherJob
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.MustGatherJob{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner MustGatherJob
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.MustGatherJob{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileMustGatherJob implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileMustGatherJob{}

// ReconcileMustGatherJob reconciles a MustGatherJob object
type ReconcileMustGatherJob struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a MustGatherJob object and makes changes based on the state read
// and what is in the MustGatherJob.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileMustGatherJob) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling MustGatherJob")

	// Fetch the MustGatherJob instance
	instance := &operatorv1alpha1.MustGatherJob{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Define a new must gather job
	job := newMustGatherJob(instance)

	// Set MustGatherJob instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, job, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Pod already exists
	found := &batchv1.Job{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new must gahter job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
		err = r.client.Create(context.TODO(), job)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Job already exists - don't requeue
	reqLogger.Info("Skip reconcile: Jod already exists", "Job.Namespace", found.Namespace, "Job.Name", found.Name)
	return reconcile.Result{}, nil
}

// newMustGatherJob returns job with the same name/namespace as the cr
func newMustGatherJob(cr *operatorv1alpha1.MustGatherJob) *batchv1.Job {
	var backoffLimit = int32(4)

	appName := cr.Name

	serviceAccountName := "default"
	if len(cr.Spec.ServiceAccountName) > 0 {
		serviceAccountName = cr.Spec.ServiceAccountName
	}

	image := os.Getenv("MUST_GATHER_IMAGE")
	if len(cr.Spec.Image.Repository) > 0 && len(cr.Spec.Image.Tag) > 0 {
		image = cr.Spec.Image.Repository + ":" + cr.Spec.Image.Tag
	}

	command := []string{"gather"}
	if len(cr.Spec.MustGatherCommand) > 0 {
		command = strings.Split(cr.Spec.MustGatherCommand, " ")
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      appName,
			Namespace: cr.Namespace,
			Labels:    labelsForMustGatherJob("must-gather-job", cr.Name),
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: &backoffLimit,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        cr.Name,
					Labels:      labelsForMustGatherJob("must-gather-job", cr.Name),
					Annotations: annotationsForMustGatherJob(),
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      "Never",
					ServiceAccountName: serviceAccountName,
					Affinity: &corev1.Affinity{
						PodAffinity: &corev1.PodAffinity{
							RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
								{
									LabelSelector: &metav1.LabelSelector{
										MatchLabels: map[string]string{
											"app.kubernetes.io/name":       "must-gather-service",
											"app.kubernetes.io/instance":   "must-gather-service",
											"app.kubernetes.io/managed-by": "",
										},
									},
									TopologyKey: "kubernetes.io/hostname",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            appName,
							Image:           image,
							ImagePullPolicy: corev1.PullPolicy(cr.Spec.Image.PullPolicy),
							Command:         command,
							Env: []corev1.EnvVar{
								{
									Name:  "FROM_OPERATOR",
									Value: "1",
								},
								{
									Name:  "INSTANCE_NAME",
									Value: appName,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "must-gather-pvc",
									MountPath: "/must-gather",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "must-gather-pvc",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
									ClaimName: "must-gather-pvc",
								},
							},
						},
					},
				},
			},
		},
	}

	if len(cr.Spec.MustGatherCommand) == 0 {
		job.Spec.Template.Spec.Containers[0].VolumeMounts = append(job.Spec.Template.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "must-gather-config",
			MountPath: "/usr/bin/gather_config",
			SubPath:   "gather_config",
		})
		job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, corev1.Volume{
			Name: "must-gather-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cr.Spec.MustGatherConfigName,
					},
				},
			},
		})
	}

	return job
}

func labelsForMustGatherJob(name string, releaseName string) map[string]string {
	return map[string]string{
		"app":                          name,
		"release":                      releaseName,
		"app.kubernetes.io/name":       name,
		"app.kubernetes.io/instance":   releaseName,
		"app.kubernetes.io/managed-by": "",
	}
}

func annotationsForMustGatherJob() map[string]string {
	return map[string]string{
		"productName":   "IBM Cloud Platform Common Services",
		"productID":     "068a62892a1e4db39641342e592daa25",
		"productMetric": "FREE",
	}
}
