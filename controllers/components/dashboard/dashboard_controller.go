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

package dashboard

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	componentsv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/components/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/deploy"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/render"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/updatestatus"
	odhrec "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/reconciler"
	odhdeploy "github.com/opendatahub-io/opendatahub-operator/v2/pkg/deploy"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/manifests/kustomize"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
)

const (
	ComponentName = "dashboard"
)

var (
	ComponentNameUpstream = ComponentName
	PathUpstream          = odhdeploy.DefaultManifestPath + "/" + ComponentNameUpstream + "/odh"

	ComponentNameDownstream = "rhods-dashboard"
	PathDownstream          = odhdeploy.DefaultManifestPath + "/" + ComponentNameUpstream + "/rhoai"
	PathSelfDownstream      = PathDownstream + "/onprem"
	PathManagedDownstream   = PathDownstream + "/addon"
	OverridePath            = ""
	DefaultPath             = ""

	dashboardID = types.NamespacedName{Name: componentsv1.DashboardInstanceName}

	adminGroups = map[cluster.Platform]string{
		cluster.SelfManagedRhods: "rhods-admins",
		cluster.ManagedRhods:     "dedicated-admins",
		cluster.OpenDataHub:      "odh-admins",
		cluster.Unknown:          "odh-admins",
	}

	sectionTitle = map[cluster.Platform]string{
		cluster.SelfManagedRhods: "OpenShift Self Managed Services",
		cluster.ManagedRhods:     "OpenShift Managed Services",
		cluster.OpenDataHub:      "OpenShift Open Data Hub",
		cluster.Unknown:          "OpenShift Open Data Hub",
	}

	baseConsoleURL = map[cluster.Platform]string{
		cluster.SelfManagedRhods: "https://rhods-dashboard-",
		cluster.ManagedRhods:     "https://rhods-dashboard-",
		cluster.OpenDataHub:      "https://odh-dashboard-",
		cluster.Unknown:          "https://odh-dashboard-",
	}

	manifestPaths = map[cluster.Platform]string{
		cluster.SelfManagedRhods: PathDownstream + "/onprem",
		cluster.ManagedRhods:     PathDownstream + "/addon",
		cluster.OpenDataHub:      PathUpstream,
		cluster.Unknown:          PathUpstream,
	}

	imagesMap = map[string]string{
		"odh-dashboard-image": "RELATED_IMAGE_ODH_DASHBOARD_IMAGE",
	}
)

// +kubebuilder:rbac:groups=components.opendatahub.io,resources=dashboards,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=components.opendatahub.io,resources=dashboards/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=components.opendatahub.io,resources=dashboards/finalizers,verbs=update
// +kubebuilder:rbac:groups="opendatahub.io",resources=odhdashboardconfigs,verbs=create;get;patch;watch;update;delete;list
// +kubebuilder:rbac:groups="console.openshift.io",resources=odhquickstarts,verbs=create;get;patch;list;delete
// +kubebuilder:rbac:groups="dashboard.opendatahub.io",resources=odhdocuments,verbs=create;get;patch;list;delete
// +kubebuilder:rbac:groups="dashboard.opendatahub.io",resources=odhapplications,verbs=create;get;patch;list;delete
// +kubebuilder:rbac:groups="dashboard.opendatahub.io",resources=acceleratorprofiles,verbs=create;get;patch;list;delete
// +kubebuilder:rbac:groups="operators.coreos.com",resources=clusterserviceversions,verbs=get;list;watch;delete;update
// +kubebuilder:rbac:groups="operators.coreos.com",resources=customresourcedefinitions,verbs=create;get;patch;delete
// +kubebuilder:rbac:groups="operators.coreos.com",resources=subscriptions,verbs=get;list;watch;delete
// +kubebuilder:rbac:groups="operators.coreos.com",resources=operatorconditions,verbs=get;list;watch
// +kubebuilder:rbac:groups="user.openshift.io",resources=groups,verbs=get;create;list;watch;patch;delete
// +kubebuilder:rbac:groups="console.openshift.io",resources=consolelinks,verbs=create;get;patch;delete
// +kubebuilder:rbac:groups="authorization.openshift.io",resources=roles,verbs=*
// +kubebuilder:rbac:groups="authorization.openshift.io",resources=rolebindings,verbs=*
// +kubebuilder:rbac:groups="authorization.openshift.io",resources=clusterroles,verbs=*
// +kubebuilder:rbac:groups="authorization.openshift.io",resources=clusterrolebindings,verbs=*

// +kubebuilder:rbac:groups="argoproj.io",resources=workflows,verbs=*

// +kubebuilder:rbac:groups="apps",resources=statefulsets,verbs=*

// +kubebuilder:rbac:groups="apps",resources=replicasets,verbs=*

// +kubebuilder:rbac:groups="apps",resources=deployments/finalizers,verbs=*
// +kubebuilder:rbac:groups="core",resources=deployments,verbs=*
// +kubebuilder:rbac:groups="apps",resources=deployments,verbs=*
// +kubebuilder:rbac:groups="*",resources=deployments,verbs=*
// +kubebuilder:rbac:groups="extensions",resources=deployments,verbs=*

// +kubebuilder:rbac:groups="apiextensions.k8s.io",resources=customresourcedefinitions,verbs=get;list;watch;create;patch;delete

// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=validatingwebhookconfigurations,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="admissionregistration.k8s.io",resources=mutatingwebhookconfigurations,verbs=create;delete;list;update;watch;patch;get

// +kubebuilder:rbac:groups="*",resources=statefulsets,verbs=create;update;get;list;watch;patch;delete

// +kubebuilder:rbac:groups="*",resources=replicasets,verbs=*

func NewDashboardReconciler(ctx context.Context, mgr ctrl.Manager) error {
	var componentLabelPredicate predicate.Predicate
	var componentEventHandler handler.EventHandler

	switch cluster.GetRelease().Name {
	case cluster.SelfManagedRhods, cluster.ManagedRhods:
		componentLabelPredicate = dashboardWatchPredicate(ComponentNameUpstream)
		componentEventHandler = watchDashboardResources(ComponentNameUpstream)
	default:
		componentLabelPredicate = dashboardWatchPredicate(ComponentNameDownstream)
		componentEventHandler = watchDashboardResources(ComponentNameDownstream)
	}

	forOpts := builder.WithPredicates(predicate.Or(
		predicate.GenerationChangedPredicate{},
		predicate.LabelChangedPredicate{},
		predicate.AnnotationChangedPredicate{},
	))

	_, err := odhrec.ComponentReconcilerFor[*componentsv1.Dashboard](mgr, &componentsv1.Dashboard{}, forOpts).
		// operands
		Watches(&appsv1.Deployment{}, componentEventHandler, builder.WithPredicates(componentLabelPredicate)).
		Watches(&appsv1.ReplicaSet{}, componentEventHandler, builder.WithPredicates(componentLabelPredicate)).
		Watches(&corev1.Namespace{}, componentEventHandler, builder.WithPredicates(componentLabelPredicate)).
		Watches(&corev1.ConfigMap{}, componentEventHandler, builder.WithPredicates(componentLabelPredicate)).
		Watches(&corev1.PersistentVolumeClaim{}, componentEventHandler, builder.WithPredicates(componentLabelPredicate)).
		Watches(&rbacv1.ClusterRoleBinding{}, componentEventHandler, builder.WithPredicates(componentLabelPredicate)).
		Watches(&rbacv1.ClusterRole{}, componentEventHandler, builder.WithPredicates(componentLabelPredicate)).
		Watches(&rbacv1.Role{}, componentEventHandler, builder.WithPredicates(componentLabelPredicate)).
		Watches(&rbacv1.RoleBinding{}, componentEventHandler, builder.WithPredicates(componentLabelPredicate)).
		Watches(&corev1.ServiceAccount{}, componentEventHandler, builder.WithPredicates(componentLabelPredicate)).
		// misc
		WithComponentName(ComponentName).
		// actions
		WithActionFn(initialize).
		WithActionFn(devFlags).
		WithAction(render.New(
			render.WithManifestsOptions(
				kustomize.WithEngineRenderOpts(
					kustomize.WithLabel(labels.ComponentName, ComponentName),
				),
			),
		)).
		WithActionFn(customizeResources).
		WithAction(deploy.New(
			deploy.WithMode(deploy.ModeSSA),
			deploy.WithFieldOwner(ComponentName),
		)).
		WithAction(updatestatus.New(
			updatestatus.WithSelectorLabel(labels.ComponentName, ComponentName),
		)).
		Build(ctx)

	if err != nil {
		return fmt.Errorf("could not create the dashboard controller: %w", err)
	}

	return nil
}

//nolint:ireturn
func watchDashboardResources(componentName string) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, a client.Object) []reconcile.Request {
		switch {
		case a.GetLabels()[labels.ODH.Component(componentName)] == "true":
			return []reconcile.Request{{NamespacedName: dashboardID}}
		case a.GetLabels()[labels.ComponentName] == ComponentName:
			return []reconcile.Request{{NamespacedName: dashboardID}}
		}

		return nil
	})
}

func dashboardWatchPredicate(componentName string) predicate.Funcs {
	label := labels.ODH.Component(componentName)
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			labelList := e.Object.GetLabels()

			if value, exist := labelList[labels.ComponentName]; exist && value == ComponentName {
				return true
			}
			if value, exist := labelList[label]; exist && value == "true" {
				return true
			}

			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			oldLabels := e.ObjectOld.GetLabels()

			if value, exist := oldLabels[labels.ComponentName]; exist && value == ComponentName {
				return true
			}
			if value, exist := oldLabels[label]; exist && value == "true" {
				return true
			}

			newLabels := e.ObjectNew.GetLabels()

			if value, exist := newLabels[labels.ComponentName]; exist && value == ComponentName {
				return true
			}
			if value, exist := newLabels[label]; exist && value == "true" {
				return true
			}

			return false
		},
	}
}
