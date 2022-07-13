package kustomize

import (
	"github.com/opendatahub-io/opendatahub-operator/pkg/manifests"
)

type KustomizeConfig struct {
	manifestType string
	component    *manifests.ComponentMap
}

func GetOdhDeploymentConfig(cm *manifests.ComponentMap) (manifests.OdhDeploymentConfig, error) {
	kc := &KustomizeConfig{
		manifestType: manifests.KUSTOMIZE,
		component:    cm,
	}
	return kc, nil
}

func (kc *KustomizeConfig) ApplyConfig() {
}

func (kc *KustomizeConfig) DeleteConfig() {

}
