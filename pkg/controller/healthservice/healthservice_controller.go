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

	operatorv1alpha1 "github.com/IBM/ibm-healthcheck-operator/pkg/apis/operator/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_healthservice")

var (
	// watchedResources contains the resources we will watch and reconcile when changed
	watchedResources = []schema.GroupVersionKind{
		{Group: "", Version: "v1", Kind: "ConfigMap"},
	}

	ownedResourcePredicates = predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		GenericFunc: func(_ event.GenericEvent) bool {
			// no action
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			// only handle delete event in case user accidentally removed the managed resource.
			return true
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			return false
		},
	}
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new HealthService Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileHealthService{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("healthservice-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource HealthService
	err = c.Watch(&source.Kind{Type: &operatorv1alpha1.HealthService{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	//watch for changes to operand resources
	err = watchOperandResources(c)
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner HealthService
	// Deployment
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.HealthService{},
	})
	if err != nil {
		return err
	}

	// Service
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.HealthService{},
	})
	if err != nil {
		return err
	}

	// Ingress
	err = c.Watch(&source.Kind{Type: &networkingv1.Ingress{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.HealthService{},
	})
	if err != nil {
		return err
	}

	// Configmap
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &operatorv1alpha1.HealthService{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileHealthService implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileHealthService{}

// ReconcileHealthService reconciles a HealthService object
type ReconcileHealthService struct {
	// TODO: Clarify the split client
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a HealthService object and makes changes based on the state read
// and what is in the HealthService.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a HealthService Deployment for each HealthService CR
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileHealthService) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling HealthService")

	// Fetch the HealthService instance
	healthService := &operatorv1alpha1.HealthService{}
	err := r.client.Get(context.TODO(), request.NamespacedName, healthService)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			reqLogger.Info("HealthService resource not found. Ignoring since object must be deleted")
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		reqLogger.Error(err, "Failed to get HealthService")
		return reconcile.Result{}, err
	}

	if err = r.createOrUpdateMemcachedDeploy(healthService); err != nil {
		reqLogger.Error(err, "Failed to create or update Deployment for memcached")
		return reconcile.Result{}, err
	}

	if err = r.createOrUpdateMemcachedService(healthService); err != nil {
		reqLogger.Error(err, "Failed to create or update Service for memcached")
		return reconcile.Result{}, err
	}

	if err = r.createOrUpdateHealthServiceConfigmap(healthService); err != nil {
		reqLogger.Error(err, "Failed to create or update configmap for health service")
		return reconcile.Result{}, err
	}

	if err = r.createOrUpdateHealthServiceDeploy(healthService); err != nil {
		reqLogger.Error(err, "Failed to create or update Deployment for health service")
		return reconcile.Result{}, err
	}

	if err = r.createOrUpdateHealthServiceService(healthService); err != nil {
		reqLogger.Error(err, "Failed to create or update Service for health service")
		return reconcile.Result{}, err
	}

	if err = r.createOrUpdateHealthServiceIngress(healthService); err != nil {
		reqLogger.Error(err, "Failed to create or update Ingress for health service")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

// Watch configmap resources managed by the operator
func watchOperandResources(c controller.Controller) error {
	for _, t := range watchedResources {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Kind:    t.Kind,
			Group:   t.Group,
			Version: t.Version,
		})
		err := c.Watch(&source.Kind{Type: u}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &operatorv1alpha1.HealthService{},
		}, ownedResourcePredicates)

		if err != nil {
			klog.Errorf("Could not create watch for %s/%s/%s: %s.", t.Kind, t.Group, t.Version, err)
		}
	}
	return nil
}
