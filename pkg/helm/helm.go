package helm

import (
	"github.com/opendatahub-io/opendatahub-operator/pkg/manifests"
)

type HelmConfig struct {
	manifestType string
	component    *manifests.ComponentMap
}

func GetOdhDeploymentConfig(cm *manifests.ComponentMap) (manifests.OdhDeploymentConfig, error) {
	hc := &HelmConfig{
		manifestType: manifests.HELM,
		component:    cm,
	}
	return hc, nil
}

func (hc *HelmConfig) ApplyConfig() {
}

func (hc *HelmConfig) DeleteConfig() {

}
