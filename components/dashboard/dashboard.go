package dashboard

import (
	"github.com/opendatahub-io/opendatahub-operator/components"
	"github.com/opendatahub-io/opendatahub-operator/pkg/deploy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ComponentName = "odh-dashboard"
	Path          = "/opt/odh-manifests/odh-dashboard/base"
)

type Dashboard struct {
	components.Component `json:""`
}

// Verifies that Dashboard implements ComponentInterface
var _ components.ComponentInterface = (*Dashboard)(nil)

func (d *Dashboard) ReconcileComponent(owner metav1.Object, client client.Client, scheme *runtime.Scheme, enabled bool, namespace string) error {

	// TODO: Add any additional tasks if required when reconciling component
	err := deploy.DeployManifestsFromPath(owner, client,
		Path,
		namespace,
		scheme, enabled)
	return err

}

func (in *Dashboard) DeepCopyInto(out *Dashboard) {
	*out = *in
	out.Component = in.Component
}
