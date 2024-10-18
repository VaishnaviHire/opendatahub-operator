package dashboard

import (
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	componentsv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/components/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	odhtypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	odhdeploy "github.com/opendatahub-io/opendatahub-operator/v2/pkg/deploy"
)

func initialize(ctx context.Context, rr *odhtypes.ReconciliationRequest) error {
	// Implement initialization logic
	log := logf.FromContext(ctx).WithName(ComponentNameUpstream)

	rr.Manifests = []odhtypes.ManifestInfo{defaultManifestInfo(rr.Platform)}

	if err := odhdeploy.ApplyParams(rr.Manifests[0].ManifestsPath(), imagesMap); err != nil {
		log.Error(err, "failed to update image", "path", rr.Manifests[0].ManifestsPath())
	}

	extraParamsMap, err := updateKustomizeVariable(ctx, rr.Client, rr.Platform, &rr.DSCI.Spec)
	if err != nil {
		return errors.New("failed to set variable for extraParamsMap")
	}

	if err := odhdeploy.ApplyParams(rr.Manifests[0].ManifestsPath(), nil, extraParamsMap); err != nil {
		return fmt.Errorf("failed to update params.env  from %s : %w", rr.Manifests[0].ManifestsPath(), err)
	}

	return nil
}

func devFlags(ctx context.Context, rr *odhtypes.ReconciliationRequest) error {
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
		if err := odhdeploy.DownloadManifests(ctx, ComponentNameUpstream, manifestConfig); err != nil {
			return err
		}
		if manifestConfig.SourcePath != "" {
			rr.Manifests[0].Path = odhdeploy.DefaultManifestPath
			rr.Manifests[0].ContextDir = ComponentNameUpstream
			rr.Manifests[0].SourcePath = manifestConfig.SourcePath
		}
	}

	return nil
}

func customizeResources(ctx context.Context, rr *odhtypes.ReconciliationRequest) error {
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
