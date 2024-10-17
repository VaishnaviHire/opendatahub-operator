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
	"errors"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	componentsv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/components/v1"
	dscv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/datasciencecluster/v1"
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions"
	odhrec "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/reconciler"
	odhtypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/deploy"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/manifests/kustomize"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
)

const (
	ComponentName = "dashboard"
)

var (
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
	r, err := odhrec.NewComponentReconciler[*componentsv1.Dashboard](ctx, mgr, ComponentName)
	if err != nil {
		return err
	}

	actionCtx := logf.IntoContext(ctx, r.Log)
	d := Dashboard{}

	// Add Dashboard-specific actions
	r.AddAction(actions.NewActionFn(actionCtx, d.initialize))
	r.AddAction(actions.NewActionFn(actionCtx, d.devFlags))

	r.AddAction(actions.NewRenderManifestsAction(
		actionCtx,
		actions.WithRenderManifestsAllowList(false),
		actions.WithRenderManifestsOptions(
			kustomize.WithEngineRenderOpts(
				kustomize.WithLabel(labels.ComponentName, ComponentName),
			),
		),
	))

	r.AddAction(actions.NewActionFn(actionCtx, d.customizeResources))

	r.AddAction(actions.NewDeployAction(
		actionCtx,
		actions.WithDeployedMode(actions.DeployModeSSA),
		actions.WithDeployedResourceFieldOwner(ComponentName),
	))

	r.AddAction(actions.NewUpdateStatusAction(
		actionCtx,
		actions.WithUpdateStatusLabel(labels.ComponentName, ComponentName),
	))

	predicates := make([]predicate.Predicate, 0)
	switch r.Platform {
	case cluster.SelfManagedRhods, cluster.ManagedRhods:
		predicates = append(predicates, dashboardWatchPredicate(ComponentNameUpstream))
	default:
		predicates = append(predicates, dashboardWatchPredicate(ComponentNameDownstream))
	}

	eh := handler.EnqueueRequestsFromMapFunc(watchDashboardResources)
	ef := builder.WithPredicates(predicates...)

	err = ctrl.NewControllerManagedBy(mgr).
		For(&componentsv1.Dashboard{}).
		// dependants
		Watches(&appsv1.Deployment{}, eh, ef).
		Watches(&appsv1.ReplicaSet{}, eh, ef).
		Watches(&corev1.Namespace{}, eh, ef).
		Watches(&corev1.ConfigMap{}, eh, ef).
		Watches(&corev1.PersistentVolumeClaim{}, eh, ef).
		Watches(&rbacv1.ClusterRoleBinding{}, eh, ef).
		Watches(&rbacv1.ClusterRole{}, eh, ef).
		Watches(&rbacv1.Role{}, eh, ef).
		Watches(&rbacv1.RoleBinding{}, eh, ef).
		Watches(&corev1.ServiceAccount{}, eh, ef).
		// done
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
			Name: componentsv1.DashboardInstanceName,
		},
		Spec: componentsv1.DashboardSpec{
			DSCDashboard: dsc.Spec.Components.Dashboard,
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

func watchDashboardResources(_ context.Context, a client.Object) []reconcile.Request {
	if a.GetLabels()[labels.ODH.Component(ComponentNameUpstream)] == "true" || a.GetLabels()[labels.ODH.Component(ComponentNameDownstream)] == "true" {
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{Name: componentsv1.DashboardInstanceName},
		}}
	}

	return nil
}

func dashboardWatchPredicate(componentName string) predicate.Funcs {
	label := labels.ODH.Component(componentName)
	return predicate.Funcs{
		CreateFunc: func(e event.CreateEvent) bool {
			return true
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

//nolint:unused
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

// TODO added only to avoid name collision

type Dashboard struct{}

func (d Dashboard) initialize(ctx context.Context, rr *odhtypes.ReconciliationRequest) error {
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

	componentName := ComponentNameUpstream
	if rr.Platform == cluster.SelfManagedRhods || rr.Platform == cluster.ManagedRhods {
		componentName = ComponentNameDownstream
	}

	rr.Manifests = []odhtypes.ManifestInfo{{
		Path:       DefaultPath,
		ContextDir: "",
		SourcePath: "",
		RenderOpts: []kustomize.RenderOptsFn{
			kustomize.WithLabel(labels.ODH.Component(componentName), "true"),
			kustomize.WithLabel(labels.K8SCommon.PartOf, componentName),
		},
	}}


	if err := deploy.ApplyParams(DefaultPath, imageParamMap); err != nil {
		log.Error(err, "failed to update image", "path", DefaultPath)
	}

	// 2. Append or Update variable for component to consume
	extraParamsMap, err := updateKustomizeVariable(ctx, rr.Client, rr.Platform, &rr.DSCI.Spec)
	if err != nil {
		return errors.New("failed to set variable for extraParamsMap")
	}

	// 3. update params.env regardless devFlags is provided of not
	// We need this for downstream
	if err := deploy.ApplyParams(rr.Manifests[0].ManifestsPath(), nil, extraParamsMap); err != nil {
		return fmt.Errorf("failed to update params.env  from %s : %w", rr.Manifests[0].ManifestsPath(), err)
	}

	return nil
}

func (d Dashboard) devFlags(ctx context.Context, rr *odhtypes.ReconciliationRequest) error {
	dashboard, ok := rr.Instance.(*componentsv1.Dashboard)
	if !ok {
		return fmt.Errorf("resource instance %v is not a componentsv1.Dashboard)", rr.Instance)
	}

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
			rr.Manifests[0].Path = deploy.DefaultManifestPath
			rr.Manifests[0].ContextDir = ComponentNameUpstream
			rr.Manifests[0].SourcePath = manifestConfig.SourcePath
		}
	}

	return nil
}

func (d Dashboard) customizeResources(ctx context.Context, rr *odhtypes.ReconciliationRequest) error {
	switch rr.Platform {
	case cluster.SelfManagedRhods, cluster.ManagedRhods:
		if err := cluster.UpdatePodSecurityRolebinding(ctx, rr.Client, rr.DSCI.Spec.ApplicationsNamespace, "rhods-dashboard"); err != nil {
			return fmt.Errorf("failed to update PodSecurityRolebinding for rhods-dashboard: %w", err)
		}

		err := rr.AddResource(&corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "anaconda-ce-access",
				Namespace: rr.DSCI.Spec.ApplicationsNamespace,
			},
			Type: corev1.SecretTypeOpaque,
		})

		if err != nil {
			return fmt.Errorf("failed to create access-secret for anaconda: %w", err)
		}

	default:
		if err := cluster.UpdatePodSecurityRolebinding(ctx, rr.Client, rr.DSCI.Spec.ApplicationsNamespace, "odh-dashboard"); err != nil {
			return fmt.Errorf("failed to update PodSecurityRolebinding for odh-dashboard: %w", err)
		}
	}

	return nil
}
