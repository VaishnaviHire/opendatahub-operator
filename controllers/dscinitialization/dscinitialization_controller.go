/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dscinitialization

import (
	"context"
	"github.com/go-logr/logr"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	dsci "github.com/opendatahub-io/opendatahub-operator/apis/dscinitialization/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/controllers/status"
)

// DSCInitializationReconciler reconciles a DSCInitialization object
type DSCInitializationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
}

//+kubebuilder:rbac:groups=dscinitialization.opendatahub.io,resources=dscinitializations,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=dscinitialization.opendatahub.io,resources=dscinitializations/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dscinitialization.opendatahub.io,resources=dscinitializations/finalizers,verbs=update
//+kubebuilder:rbac:groups=networking.k8s.io,resources=networkpolicies,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=services;namespaces;serviceaccounts;secrets;configmaps,verbs=get;list;watch;create;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the DSCInitialization object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *DSCInitializationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	r.Log.Info("Reconciling DSCInitialization.", "DSCInitialization", req.Namespace, "Request.Name", req.Name)

	instance := &dsci.DSCInitialization{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil && apierrs.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	// Start reconciling
	if instance.Status.Conditions == nil {
		reason := status.ReconcileInit
		message := "Initializing DSCInitialization resource"
		status.SetProgressingCondition(&instance.Status.Conditions, reason, message)

		instance.Status.Phase = status.PhaseProgressing
		err = r.Client.Status().Update(ctx, instance)
		if err != nil {
			r.Log.Error(err, "Failed to add conditions to status of DSCInitialization resource.", "DSCInitialization", req.Namespace, "Request.Name", req.Name)
			return reconcile.Result{}, err
		}
	}

	// Check for list of namespaces
	for _, namespace := range instance.Spec.Namespaces {
		err = r.createOdhNamespace(namespace, ctx)
		if err != nil {
			return reconcile.Result{}, err
		}
	}

	// Finish reconciling
	reason := status.ReconcileCompleted
	message := status.ReconcileCompletedMessage
	status.SetCompleteCondition(&instance.Status.Conditions, reason, message)

	instance.Status.Phase = status.PhaseReady
	err = r.Client.Status().Update(ctx, instance)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *DSCInitializationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&dsci.DSCInitialization{}).
		Complete(r)
}
