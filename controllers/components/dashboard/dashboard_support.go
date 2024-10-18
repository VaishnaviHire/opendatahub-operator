package dashboard

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	componentsv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/components/v1"
	dscv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/datasciencecluster/v1"
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	odhtypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/manifests/kustomize"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/labels"
)

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

func defaultManifestInfo(p cluster.Platform) odhtypes.ManifestInfo {
	componentName := ComponentNameUpstream
	if p == cluster.SelfManagedRhods || p == cluster.ManagedRhods {
		componentName = ComponentNameDownstream
	}

	return odhtypes.ManifestInfo{
		Path:       manifestPaths[p],
		ContextDir: "",
		SourcePath: "",
		RenderOpts: []kustomize.RenderOptsFn{
			kustomize.WithLabel(labels.ODH.Component(componentName), "true"),
			kustomize.WithLabel(labels.K8SCommon.PartOf, componentName),
		},
	}
}

func updateKustomizeVariable(ctx context.Context, cli client.Client, platform cluster.Platform, dscispec *dsciv1.DSCInitializationSpec) (map[string]string, error) {
	consoleLinkDomain, err := cluster.GetDomain(ctx, cli)
	if err != nil {
		return nil, fmt.Errorf("error getting console route URL %s : %w", consoleLinkDomain, err)
	}

	return map[string]string{
		"admin_groups":  adminGroups[platform],
		"dashboard-url": baseConsoleURL[platform] + dscispec.ApplicationsNamespace + "." + consoleLinkDomain,
		"section-title": sectionTitle[platform],
	}, nil
}
