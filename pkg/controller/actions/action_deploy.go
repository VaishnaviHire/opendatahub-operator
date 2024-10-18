package actions

import (
	"context"
	"encoding/json"
	"fmt"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	odhClient "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/client"
	odhTypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
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

			switch r.deployMode {
			case DeployModePatch:
				err = r.apply(ctx, rr.Client, obj)
			case DeployModeSSA:
				err = r.patch(ctx, rr.Client, obj)
			default:
				err = fmt.Errorf("unsupported deploy mode %d", r.deployMode)
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
	found.SetGroupVersionKind(obj.GroupVersionKind())

	foundErr := c.Get(ctx, client.ObjectKeyFromObject(&obj), &found)
	if foundErr != nil && !k8serr.IsNotFound(foundErr) {
		return fmt.Errorf("failed to retrieve object %s/%s: %w", obj.GetNamespace(), obj.GetName(), foundErr)
	}

	switch obj.GroupVersionKind() {
	case gvk.Deployment:
		found := unstructured.Unstructured{}
		found.SetGroupVersionKind(obj.GroupVersionKind())

		err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &found)
		if err != nil && !k8serr.IsNotFound(err) {
			return fmt.Errorf("failed to retrieve Deployment %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}

		// If the resource does not exist, then don't apply the allow list
		// as the resource must be created
		if k8serr.IsNotFound(err) {
			break
		}

		// If the resource is forcefully marked as managed by the operator, don't apply
		// the allow list as the user is explicitly forcing the resource to be managed
		// i.e. to restore default values
		if found.GetAnnotations()[annotations.ManagedByODHOperator] == "true" {
			break
		}

		// To preserve backward compatibility with the current model, fields are being
		// removed, hence not included in the final PATCH. Ideally with should leverage
		// Server-Side Apply.
		//
		// Ideally deployed resources should be configured only via the platform API
		if err := RemoveDeploymentsResources(&obj); err != nil {
			return fmt.Errorf("failed to apply allow list to Deployment %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}

	default:
		// do noting
		break
	}

	if k8serr.IsNotFound(foundErr) {
		err := c.Create(ctx, &obj)
		if err != nil {
			return fmt.Errorf("failed to create object %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
	} else {
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
			return fmt.Errorf("failed to patch object %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}
	}

	return nil
}

func (r *DeployAction) apply(ctx context.Context, c *odhClient.Client, obj unstructured.Unstructured) error {
	switch obj.GroupVersionKind() {
	case gvk.Deployment:
		found := unstructured.Unstructured{}
		found.SetGroupVersionKind(obj.GroupVersionKind())

		err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &found)
		if err != nil && !k8serr.IsNotFound(err) {
			return fmt.Errorf("failed to retrieve Deployment %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}

		// For deployments, we allow the user to change some parameters, such as
		// container resources and replicas except:
		// - If the resource does not exist (the resource must be created)
		// - If the resource is forcefully marked as managed by the operator via
		//   annotations (i.e. to bring it back to the default values)
		if k8serr.IsNotFound(err) || found.GetAnnotations()[annotations.ManagedByODHOperator] == "true" {
			break
		}

		// To preserve backward compatibility with the current model, fields are being
		// merged from an existing Deployment (if it exists) to the rendered manifest,
		// hence the current value is preserved [1].
		//
		// Ideally deployed resources should be configured only via the platform API
		//
		// [1] https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts
		if err := MergeDeployments(&found, &obj); err != nil {
			return fmt.Errorf("failed to merge Deployment %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
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
		return fmt.Errorf("apply failed %s: %w", obj.GroupVersionKind(), err)
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
