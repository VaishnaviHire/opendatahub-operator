package pkg

import (
	"github.com/opendatahub-io/opendatahub-operator/pkg/helm"
	"github.com/opendatahub-io/opendatahub-operator/pkg/kustomize"
	"github.com/opendatahub-io/opendatahub-operator/pkg/manifests"
)

// PackageManagers
const (
	HELM      = "helm"
	KUSTOMIZE = "kustomize"
)

// Add to this map any additional package managers
func getSupportedPackageManagers() map[string]manifests.OdhDeploymentConfig {
	packagemanagers := map[string]manifests.OdhDeploymentConfig{
		HELM:      &helm.HelmConfig{},
		KUSTOMIZE: &kustomize.KustomizeConfig{},
	}
	return packagemanagers
}
