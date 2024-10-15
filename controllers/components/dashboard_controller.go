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

package components

import (
	"context"
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/opendatahub-io/opendatahub-operator/v2/apis/components"
	componentsv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/components/v1"
	dscv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/datasciencecluster/v1"
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/deploy"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
)

var (
	DashboardInstanceName = "default-dashboard"
	ComponentName         = "dashboard"
	ComponentNameUpstream = ComponentName
	PathUpstream          = deploy.DefaultManifestPath + "/" + ComponentNameUpstream + "/odh"

	ComponentNameDownstream = "rhods-dashboard"
	PathDownstream          = deploy.DefaultManifestPath + "/" + ComponentNameUpstream + "/rhoai"
	PathSelfDownstream      = PathDownstream + "/onprem"
	PathManagedDownstream   = PathDownstream + "/addon"
	OverridePath            = ""
	DefaultPath             = ""
)

func NewDashboardReconciler(ctx context.Context, mgr ctrl.Manager) error {
	r, err := NewBaseReconciler[*componentsv1.Dashboard](ctx, mgr, ComponentName)
	if err != nil {
		return err
	}

	// Add Dashboard-specific actions
	r.AddAction(&InitializeAction{BaseAction{Log: mgr.GetLogger().WithName("actions").WithName("initialize")}})
	r.AddAction(&SupportDevFlagsAction{BaseAction{Log: mgr.GetLogger().WithName("actions").WithName("devFlags")}})
	r.AddAction(&CleanupOAuthClientAction{BaseAction{Log: mgr.GetLogger().WithName("actions").WithName("cleanup")}})
	r.AddAction(&DeployComponentAction{BaseAction{Log: mgr.GetLogger().WithName("actions").WithName("deploy")}})
	r.AddAction(&UpdateStatusAction{BaseAction{Log: mgr.GetLogger().WithName("actions").WithName("update-status")}})

	r.AddFinalizer(&DeleteResourcesAction{
		BaseAction: BaseAction{
			Log: mgr.GetLogger().WithName("finalizers").WithName("cleanup"),
		},
		// include only the types that must be deleted
		Types: []client.Object{
			&corev1.Secret{},
		},
		Labels: map[string]string{
			"app.opendatahub.io/dashboard": "true",
		},
	})

	eh := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, a client.Object) []reconcile.Request {
		return watchDashboardResources(ctx, a)
	})

	err = ctrl.NewControllerManagedBy(mgr).
		For(&componentsv1.Dashboard{}).
		// dependants
		Watches(&appsv1.Deployment{}, eh).
		Watches(&appsv1.ReplicaSet{}, eh).
		Watches(&corev1.Namespace{}, eh).
		Watches(&corev1.ConfigMap{}, eh).
		Watches(&corev1.PersistentVolumeClaim{}, eh).
		Watches(&rbacv1.ClusterRoleBinding{}, eh).
		Watches(&rbacv1.ClusterRole{}, eh).
		Watches(&rbacv1.Role{}, eh).
		Watches(&rbacv1.RoleBinding{}, eh).
		Watches(&corev1.ServiceAccount{}, eh).
		// shared filter
		WithEventFilter(dashboardPredicates).
		Complete(r)

	if err != nil {
		return fmt.Errorf("could not create the dashboard controller: %w", err)
	}

	return nil
}

func CreateDashboardInstance(dsc *dscv1.DataScienceCluster) *componentsv1.Dashboard {
	return &componentsv1.Dashboard{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Dashboard",
			APIVersion: "components.opendatahub.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: DashboardInstanceName,
		},
		Spec: componentsv1.DashboardSpec{
			DSCDashboard: componentsv1.DSCDashboard{
				Component: components.Component{
					ManagementState: dsc.Spec.Components.Dashboard.ManagementState,
					DevFlags:        dsc.Spec.Components.Dashboard.DevFlags,
				},
			},
		},
	}
}

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

func watchDashboardResources(ctx context.Context, a client.Object) []reconcile.Request {

	if a.GetLabels()["app.opendatahub.io/dashboard"] == "true" {
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{Name: DashboardInstanceName},
		}}
	}
	return nil
}

var dashboardPredicates = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		if e.Object.GetObjectKind().GroupVersionKind().Kind == gvk.Dashboard.Kind {
			return true
		}
		// Reconcile not needed during creation
		return false
	},
	DeleteFunc: func(e event.DeleteEvent) bool {
		if e.Object.GetObjectKind().GroupVersionKind().Kind == gvk.Dashboard.Kind {
			return true
		}
		labelList := e.Object.GetLabels()
		if value, exist := labelList[labels.ODH.Component(ComponentNameUpstream)]; exist && value == "true" {
			return true
		}
		return false
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		if e.ObjectOld.GetObjectKind().GroupVersionKind().Kind == gvk.Dashboard.Kind {
			return true
		}
		labelList := e.ObjectOld.GetLabels()
		if value, exist := labelList[labels.ODH.Component(ComponentNameUpstream)]; exist && value == "true" {
			return true
		}
		return false
	},
}

func updateKustomizeVariable(ctx context.Context, cli client.Client, platform cluster.Platform, dscispec *dsciv1.DSCInitializationSpec) (map[string]string, error) {
	adminGroups := map[cluster.Platform]string{
		cluster.SelfManagedRhods: "rhods-admins",
		cluster.ManagedRhods:     "dedicated-admins",
		cluster.OpenDataHub:      "odh-admins",
		cluster.Unknown:          "odh-admins",
	}[platform]

	sectionTitle := map[cluster.Platform]string{
		cluster.SelfManagedRhods: "OpenShift Self Managed Services",
		cluster.ManagedRhods:     "OpenShift Managed Services",
		cluster.OpenDataHub:      "OpenShift Open Data Hub",
		cluster.Unknown:          "OpenShift Open Data Hub",
	}[platform]

	consoleLinkDomain, err := cluster.GetDomain(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("error getting console route URL %s : %w", consoleLinkDomain, err)
	}
	consoleURL := map[cluster.Platform]string{
		cluster.SelfManagedRhods: "https://rhods-dashboard-" + dscispec.ApplicationsNamespace + "." + consoleLinkDomain,
		cluster.ManagedRhods:     "https://rhods-dashboard-" + dscispec.ApplicationsNamespace + "." + consoleLinkDomain,
		cluster.OpenDataHub:      "https://odh-dashboard-" + dscispec.ApplicationsNamespace + "." + consoleLinkDomain,
		cluster.Unknown:          "https://odh-dashboard-" + dscispec.ApplicationsNamespace + "." + consoleLinkDomain,
	}[platform]

	return map[string]string{
		"admin_groups":  adminGroups,
		"dashboard-url": consoleURL,
		"section-title": sectionTitle,
	}, nil
}

// Action implementations

type InitializeAction struct {
	BaseAction
}

func (a *InitializeAction) Execute(ctx context.Context, rr *ReconciliationRequest) error {
	// Implement initialization logic
	log := logf.FromContext(ctx).WithName(ComponentNameUpstream)

	imageParamMap := map[string]string{
		"odh-dashboard-image": "RELATED_IMAGE_ODH_DASHBOARD_IMAGE",
	}
	manifestMap := map[cluster.Platform]string{
		cluster.SelfManagedRhods: PathDownstream + "/onprem",
		cluster.ManagedRhods:     PathDownstream + "/addon",
		cluster.OpenDataHub:      PathUpstream,
		cluster.Unknown:          PathUpstream,
	}
	DefaultPath = manifestMap[rr.Platform]

	rr.Manifests = Manifests{
		Paths: manifestMap,
	}

	if err := deploy.ApplyParams(DefaultPath, imageParamMap); err != nil {
		log.Error(err, "failed to update image", "path", DefaultPath)
	}

	return nil
}

type SupportDevFlagsAction struct {
	BaseAction
}

func (a *SupportDevFlagsAction) Execute(ctx context.Context, rr *ReconciliationRequest) error {
	dashboard := rr.Instance.(*componentsv1.Dashboard)
	if dashboard.Spec.DevFlags == nil {
		return nil
	}
	// Implement devflags support logic
	// If dev flags are set, update default manifests path
	if len(dashboard.Spec.DevFlags.Manifests) != 0 {
		manifestConfig := dashboard.Spec.DevFlags.Manifests[0]
		if err := deploy.DownloadManifests(ctx, ComponentNameUpstream, manifestConfig); err != nil {
			return err
		}
		if manifestConfig.SourcePath != "" {
			// r.entryPath = filepath.Join(deploy.DefaultManifestPath, ComponentNameUpstream, manifestConfig.SourcePath)
		}
	}
	return nil
}

type CleanupOAuthClientAction struct {
	BaseAction
}

func (a *CleanupOAuthClientAction) Execute(ctx context.Context, rr *ReconciliationRequest) error {
	// Remove previous oauth-client secrets
	// Check if component is going from state of `Not Installed --> Installed`
	// Assumption: Component is currently set to enabled
	name := "dashboard-oauth-client"

	// r.Log.Info("Cleanup any left secret")
	// Delete client secrets from previous installation
	oauthClientSecret := &corev1.Secret{}
	err := rr.Client.Get(ctx, client.ObjectKey{
		Namespace: rr.DSCI.Spec.ApplicationsNamespace,
		Name:      name,
	}, oauthClientSecret)
	if err != nil {
		if !k8serr.IsNotFound(err) {
			return fmt.Errorf("error getting secret %s: %w", name, err)
		}
	} else {
		if err := rr.Client.Delete(ctx, oauthClientSecret); err != nil {
			return fmt.Errorf("error deleting secret %s: %w", name, err)
		}
		// r.Log.Info("successfully deleted secret", "secret", name)
	}

	return nil
}

type DeployComponentAction struct {
	BaseAction
}

func (a *DeployComponentAction) Execute(ctx context.Context, rr *ReconciliationRequest) error {
	// Implement component deployment logic
	// 1. platform specific RBAC
	if rr.Platform == cluster.OpenDataHub || rr.Platform == "" {
		if err := cluster.UpdatePodSecurityRolebinding(ctx, rr.Client, rr.DSCI.Spec.ApplicationsNamespace, "odh-dashboard"); err != nil {
			return err
		}
	} else {
		if err := cluster.UpdatePodSecurityRolebinding(ctx, rr.Client, rr.DSCI.Spec.ApplicationsNamespace, "rhods-dashboard"); err != nil {
			return err
		}
	}

	// 2. Append or Update variable for component to consume
	_, err := updateKustomizeVariable(ctx, rr.Client, rr.Platform, &rr.DSCI.Spec)
	if err != nil {
		return errors.New("failed to set variable for extraParamsMap")
	}

	// 3. update params.env regardless devFlags is provided of not
	// if err := deploy.ApplyParams(r.entryPath, nil, extraParamsMap); err != nil {
	//	return fmt.Errorf("failed to update params.env  from %s : %w", r.entryPath, err)
	// }

	// common: Deploy odh-dashboard manifests
	// TODO: check if we can have the same component name odh-dashboard for both, or still keep rhods-dashboard for RHOAI
	switch rr.Platform {
	case cluster.SelfManagedRhods, cluster.ManagedRhods:
		// anaconda
		if err := cluster.CreateSecret(ctx, rr.Client, "anaconda-ce-access", rr.DSCI.Spec.ApplicationsNamespace); err != nil {
			return fmt.Errorf("failed to create access-secret for anaconda: %w", err)
		}
		// Deploy RHOAI manifests
		if err := deploy.DeployManifestsFromPath(ctx, rr.Client, rr.Instance, rr.Manifests.Paths[rr.Platform], rr.DSCI.Spec.ApplicationsNamespace, ComponentNameDownstream, true); err != nil {
			return fmt.Errorf("failed to apply manifests from %s: %w", PathDownstream, err)
		}
		a.Log.Info("apply manifests done")

		if err := cluster.WaitForDeploymentAvailable(ctx, rr.Client, ComponentNameDownstream, rr.DSCI.Spec.ApplicationsNamespace, 20, 3); err != nil {
			return fmt.Errorf("deployment for %s is not ready to server: %w", ComponentNameDownstream, err)
		}

		return nil

	default:
		// Deploy ODH manifests
		if err := deploy.DeployManifestsFromPath(ctx, rr.Client, rr.Instance, rr.Manifests.Paths[cluster.OpenDataHub], rr.DSCI.Spec.ApplicationsNamespace, ComponentNameUpstream, true); err != nil {
			return err
		}
		a.Log.Info("apply manifests done")

		if err := cluster.WaitForDeploymentAvailable(ctx, rr.Client, ComponentNameUpstream, rr.DSCI.Spec.ApplicationsNamespace, 20, 3); err != nil {
			return fmt.Errorf("deployment for %s is not ready to server: %w", ComponentNameUpstream, err)
		}
	}
	return nil
}

type UpdateStatusAction struct {
	BaseAction
}

func (a *UpdateStatusAction) Execute(ctx context.Context, rr *ReconciliationRequest) error {
	return nil
}
