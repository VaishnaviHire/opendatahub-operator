// Package dashboard provides utility functions to config Open Data Hub Dashboard: A web dashboard that displays installed Open Data Hub components with easy access to component UIs and documentation
package dashboard

import (
	"fmt"
	operatorv1 "github.com/openshift/api/operator/v1"
	"path/filepath"

	dsci "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/components"
	dscictrl "github.com/opendatahub-io/opendatahub-operator/v2/controllers/dscinitialization"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/common"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/deploy"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ComponentName          = "odh-dashboard"
	ComponentNameSupported = "rhods-dashboard"
	Path                   = deploy.DefaultManifestPath + "/" + ComponentName + "/base"
	PathSupported          = deploy.DefaultManifestPath + "/" + ComponentName + "/overlays/rhods"
	PathISVSM              = deploy.DefaultManifestPath + "/" + ComponentName + "/apps/apps-onprem"
	PathISVAddOn           = deploy.DefaultManifestPath + "/" + ComponentName + "/apps/apps-addon"
	PathOVMS               = deploy.DefaultManifestPath + "/" + ComponentName + "/modelserving"
	PathODHDashboardConfig = deploy.DefaultManifestPath + "/" + ComponentName + "/odhdashboardconfig"
	PathConsoleLink        = deploy.DefaultManifestPath + "/" + ComponentName + "/consolelink"
	PathCRDs               = deploy.DefaultManifestPath + "/" + ComponentName + "/crd"
	NameConsoleLink        = "console"
	NamespaceConsoleLink   = "openshift-console"
	PathAnaconda           = deploy.DefaultManifestPath + "/partners/anaconda/base/"
)

var imageParamMap = map[string]string{
	"odh-dashboard-image": "RELATED_IMAGE_ODH_DASHBOARD_IMAGE",
}

type Dashboard struct {
	components.Component `json:""`
}

func (d *Dashboard) OverrideManifests(platform string) error {
	// If devflags are set, update default manifests path
	if len(d.DevFlags.Manifests) != 0 {
		manifestConfig := d.DevFlags.Manifests[0]
		if err := deploy.DownloadManifests(ComponentName, manifestConfig); err != nil {
			return err
		}
		// If overlay is defined, update paths
		if platform == string(deploy.ManagedRhods) || platform == string(deploy.SelfManagedRhods) {
			defaultKustomizePath := "overlays/rhods"
			if manifestConfig.SourcePath != "" {
				defaultKustomizePath = manifestConfig.SourcePath
			}
			PathSupported = filepath.Join(deploy.DefaultManifestPath, ComponentName, defaultKustomizePath)
		} else {
			defaultKustomizePath := "base"
			if manifestConfig.SourcePath != "" {
				defaultKustomizePath = manifestConfig.SourcePath
			}
			Path = filepath.Join(deploy.DefaultManifestPath, ComponentName, defaultKustomizePath)
		}

	}
	return nil
}

func (d *Dashboard) GetComponentDevFlags() components.DevFlags {
	return d.DevFlags
}

func (d *Dashboard) SetImageParamsMap(imageMap map[string]string) map[string]string {
	imageParamMap = imageMap
	return imageParamMap
}

func (d *Dashboard) GetComponentName() string {
	return ComponentName
}

// Verifies that Dashboard implements ComponentInterface
var _ components.ComponentInterface = (*Dashboard)(nil)

func (d *Dashboard) ReconcileComponent(cli client.Client, owner metav1.Object, dscispec *dsci.DSCInitializationSpec) error {
	enabled := d.GetManagementState() == operatorv1.Managed
	monitoringEnabled := dscispec.Monitoring.ManagementState == operatorv1.Managed
	platform, err := deploy.GetPlatform(cli)
	if err != nil {
		return err
	}
	// Update Default rolebinding

	if enabled {
		// Download manifests and update paths
		if err = d.OverrideManifests(string(platform)); err != nil {
			return err
		}

		if platform == deploy.OpenDataHub || platform == "" {
			err := common.UpdatePodSecurityRolebinding(cli, []string{"odh-dashboard"}, dscispec.ApplicationsNamespace)
			if err != nil {
				return err
			}

			// Deploy CRDs for odh-dashboard
			err = deploy.DeployManifestsFromPath(cli, owner,
				PathCRDs,
				dscispec.ApplicationsNamespace,
				ComponentName,
				enabled)
			if err != nil {
				return fmt.Errorf("failed to deploy dashboard crds %s: %v", PathCRDs, err)
			}

		}
		if platform == deploy.SelfManagedRhods || platform == deploy.ManagedRhods {
			err := common.UpdatePodSecurityRolebinding(cli, []string{"rhods-dashboard"}, dscispec.ApplicationsNamespace)
			if err != nil {
				return err
			}

			// Deploy CRDs for odh-dashboard
			err = deploy.DeployManifestsFromPath(cli, owner,
				PathCRDs,
				dscispec.ApplicationsNamespace,
				ComponentNameSupported,
				enabled)
			if err != nil {
				return fmt.Errorf("failed to deploy dashboard crds %s: %v", PathCRDs, err)
			}
		}

		// Apply RHODS specific configs
		if platform == deploy.SelfManagedRhods || platform == deploy.ManagedRhods {
			// Replace admin group
			dashboardConfig := filepath.Join(PathODHDashboardConfig, "odhdashboardconfig.yaml")
			if platform == deploy.SelfManagedRhods {
				err = common.ReplaceStringsInFile(dashboardConfig, map[string]string{
					"<admin_groups>": "rhods-admins",
				})
				if err != nil {
					return err
				}
			} else if platform == deploy.ManagedRhods {
				err = common.ReplaceStringsInFile(dashboardConfig, map[string]string{
					"<admin_groups>": "dedicated-admins",
				})
				if err != nil {
					return err
				}
			}

			// Create ODHDashboardConfig if it doesn't exist already
			err = deploy.DeployManifestsFromPath(cli, owner, PathODHDashboardConfig, dscispec.ApplicationsNamespace, ComponentNameSupported, enabled)
			if err != nil {
				return fmt.Errorf("failed to set dashboard config from %s: %w", PathODHDashboardConfig, err)
			}

			// Apply modelserving config
			err = deploy.DeployManifestsFromPath(cli, owner, PathOVMS, dscispec.ApplicationsNamespace, ComponentNameSupported, enabled)
			if err != nil {
				return fmt.Errorf("failed to set dashboard OVMS from %s: %w", PathOVMS, err)
			}

			// Apply anaconda config
			err = common.CreateSecret(cli, "anaconda-ce-access", dscispec.ApplicationsNamespace)
			if err != nil {
				return fmt.Errorf("failed to create access-secret for anaconda: %w", err)
			}
			err = deploy.DeployManifestsFromPath(cli, owner, PathAnaconda, dscispec.ApplicationsNamespace, ComponentNameSupported, enabled)
			if err != nil {
				return fmt.Errorf("failed to deploy anaconda resources from %s: %w", PathAnaconda, err)
			}
		}

		// Update image parameters (ODH does not use this solution, only downstream)
		if dscispec.DevFlags.ManifestsUri == "" {
			if err := deploy.ApplyImageParams(PathSupported, imageParamMap); err != nil {
				return err
			}
		}
	}

	// Deploy odh-dashboard manifests
	if platform == deploy.OpenDataHub || platform == "" {
		err = deploy.DeployManifestsFromPath(cli, owner, Path, dscispec.ApplicationsNamespace, ComponentName, enabled)
		if err != nil {
			return err
		}
	} else if platform == deploy.SelfManagedRhods || platform == deploy.ManagedRhods {
		// Apply authentication overlay
		err = deploy.DeployManifestsFromPath(cli, owner, PathSupported, dscispec.ApplicationsNamespace, ComponentNameSupported, enabled)
		if err != nil {
			return err
		}
	}

	// ISV handling
	pathConsoleLink := filepath.Join(PathConsoleLink, "consolelink.yaml")
	switch platform {
	case deploy.SelfManagedRhods:
		err = deploy.DeployManifestsFromPath(cli, owner, PathISVSM, dscispec.ApplicationsNamespace, ComponentNameSupported, enabled)
		if err != nil {
			return fmt.Errorf("failed to set dashboard ISV from %s: %w", PathISVSM, err)
		}
		// ConsoleLink handling
		consolelinkDomain, err := dscictrl.GetDomain(cli)
		if err != nil {
			return fmt.Errorf("error getting console route URL : %v", err)
		} else {
			err = common.ReplaceStringsInFile(pathConsoleLink, map[string]string{
				"<rhods-dashboard-url>": "https://rhods-dashboard-" + dscispec.ApplicationsNamespace + "." + consolelinkDomain,
				"<section-title>":       "OpenShift Self Managed Services",
			})
			if err != nil {
				return fmt.Errorf("error replacing with correct dashboard url for ConsoleLink: %v", err)
			}
		}
		err = deploy.DeployManifestsFromPath(cli, owner, PathConsoleLink, dscispec.ApplicationsNamespace, ComponentNameSupported, enabled)
		if err != nil {
			return fmt.Errorf("failed to set dashboard consolelink from %s: %w", PathConsoleLink, err)
		}
		// CloudService Monitoring handling
		if err := d.UpdatePrometheusConfig(cli, enabled && monitoringEnabled, ComponentName); err != nil {
			return err
		}
		if err = deploy.DeployManifestsFromPath(cli, owner,
			filepath.Join(deploy.DefaultManifestPath, "monitoring", "prometheus", "apps"),
			dscispec.Monitoring.Namespace,
			ComponentName+"prometheus", true); err != nil {
			return err
		}
		return nil
	case deploy.ManagedRhods:
		err = deploy.DeployManifestsFromPath(cli, owner, PathISVAddOn, dscispec.ApplicationsNamespace, ComponentNameSupported, enabled)
		if err != nil {
			return fmt.Errorf("failed to set dashboard ISV from %s: %w", PathISVAddOn, err)
		}
		// ConsoleLink handling
		consolelinkDomain, err := dscictrl.GetDomain(cli)
		if err != nil {
			return fmt.Errorf("error getting console route URL : %v", err)
		} else {
			err = common.ReplaceStringsInFile(pathConsoleLink, map[string]string{
				"<rhods-dashboard-url>": "https://rhods-dashboard-" + dscispec.ApplicationsNamespace + "." + consolelinkDomain,
				"<section-title>":       "OpenShift Managed Services",
			})
			if err != nil {
				return fmt.Errorf("Error replacing with correct dashboard url for ConsoleLink: %v", err)
			}
			err = deploy.DeployManifestsFromPath(cli, owner,
				PathConsoleLink,
				dscispec.ApplicationsNamespace,
				ComponentNameSupported, enabled)
			if err != nil {
				return fmt.Errorf("failed to set dashboard consolelink from %s", PathConsoleLink)
			}
		}
		// CloudService Monitoring handling
		if err := d.UpdatePrometheusConfig(cli, enabled && monitoringEnabled, ComponentName); err != nil {
			return err
		}
		if err = deploy.DeployManifestsFromPath(cli, owner,
			filepath.Join(deploy.DefaultManifestPath, "monitoring", "prometheus", "apps"),
			dscispec.Monitoring.Namespace,
			ComponentName+"prometheus", true); err != nil {
			return err
		}
		return nil
	default:
		return nil
	}
}

func (in *Dashboard) DeepCopyInto(out *Dashboard) {
	*out = *in
	out.Component = in.Component
}
