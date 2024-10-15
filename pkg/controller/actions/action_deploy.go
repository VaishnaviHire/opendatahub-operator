package actions

import (
	"context"
	"encoding/json"
	odhClient "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/client"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	odhTypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

type DeployMode int

const (
	DeployModePatch DeployMode = iota
	DeployModeSSA
)

const (
	DeployActionName = "deploy"
)

// DeployAction deploys the resources that are included in the ReconciliationRequest using
// the same create or patch machinery implemented as part of deploy.DeployManifestsFromPath
//
// TODO: we should support full Server-Side Apply.
type DeployAction struct {
	BaseAction
	fieldOwner string
	deployMode DeployMode
}

type DeployActionOpts func(*DeployAction)

func WithDeployedResourceFieldOwner(value string) DeployActionOpts {
	return func(action *DeployAction) {
		action.fieldOwner = value
	}
}
func WithDeployedMode(value DeployMode) DeployActionOpts {
	return func(action *DeployAction) {
		action.deployMode = value
	}
}

func (r *DeployAction) Execute(ctx context.Context, rr *odhTypes.ReconciliationRequest) error {
	for i := range rr.Resources {
		obj := rr.Resources[i]

		switch obj.GroupVersionKind() {
		case gvk.CustomResourceDefinition:
			// No need to set owner reference for CRDs as they should
			// not be deleted when the parent is deleted
			break
		case gvk.OdhDashboardConfig:
			// We want the OdhDashboardConfig resource that is shipped
			// as part of dashboard's manifest to stay on the cluster
			// so, no need to set owner reference
			break
		default:
			if err := ctrl.SetControllerReference(rr.Instance, &obj, rr.Client.Scheme()); err != nil {
				return err
			}
		}

		switch obj.GroupVersionKind() {
		case gvk.OdhDashboardConfig:
			// The OdhDashboardConfig should only be created once, or
			// re-created if no existing, but should not be updated
			err := rr.Client.Create(ctx, &rr.Resources[i])
			if err != nil && !k8serr.IsAlreadyExists(err) {
				return err
			}
		default:
			var err error

			if r.deployMode == DeployModeSSA {
				// full server side apply
				err = r.apply(ctx, rr.Client, obj)
			} else {
				// classic behavior, default
				err = r.patch(ctx, rr.Client, obj)
			}

			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *DeployAction) patch(ctx context.Context, c *odhClient.Client, obj unstructured.Unstructured) error {
	found := unstructured.Unstructured{}
	err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &found)
	if err != nil {
		return err
	}

	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	err = c.Patch(
		ctx,
		&found,
		client.RawPatch(types.ApplyPatchType, data),
		client.ForceOwnership,
		client.FieldOwner(r.fieldOwner),
	)

	if err != nil {
		return err
	}

	return nil
}

func (r *DeployAction) apply(ctx context.Context, c *odhClient.Client, obj unstructured.Unstructured) error {
	switch obj.GroupVersionKind() {
	case gvk.Deployment:
		found := unstructured.Unstructured{}
		err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &found)
		if err != nil && !k8serr.IsNotFound(err) {
			return err
		}

		// For deployments, we allow the user to change some parameters, such as
		// container resources and replicas except:
		// - If the resource does not exist (the resource must be created)
		// - If the resource is forcefully marked as managed by the operator via
		//   annotations (i.e. to bring it back to the default values)
		if k8serr.IsNotFound(err) || found.GetAnnotations()[annotations.ManagedByODHOperator] == "true" {
			break
		}

		if err := MergeDeployments(&found, &obj); err != nil {
			return err
		}
	default:
		// do noting
		break
	}

	err := c.Apply(
		ctx,
		&obj,
		client.ForceOwnership,
		client.FieldOwner(r.fieldOwner),
	)

	if err != nil {
		return err
	}

	return nil
}

func NewDeployAction(ctx context.Context, opts ...DeployActionOpts) *DeployAction {
	action := DeployAction{
		BaseAction: BaseAction{
			Log: log.FromContext(ctx).WithName(ActionGroup).WithName(DeployActionName),
		},
		deployMode: DeployModeSSA,
	}

	for _, opt := range opts {
		opt(&action)
	}

	return &action
}
