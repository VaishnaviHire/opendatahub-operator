package manifests

import (
	odhDeployv1alpha1 "github.com/opendatahub-io/opendatahub-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ComponentMap struct {
	odhDeployment metav1.ObjectMeta
	LocalPath     string
	Version       string
	Component     odhDeployv1alpha1.Component
}

type OdhDeploymentConfig interface {
	ApplyConfig()
	DeleteConfig()
}

// Downloads the latest odhDeployment and converts it to component map
func loadConfig(odhDeployment *odhDeployv1alpha1.OdhDeployment) []ComponentMap {
	return nil
}

func applyODHDeployment(odhDeployment *odhDeployv1alpha1.OdhDeployment) error {

	return nil
}
