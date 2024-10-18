package deploy

import (
	"context"
	"encoding/json"
	"fmt"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	odhClient "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/client"
	odhTypes "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
)

type Mode int

const (
	ModePatch Mode = iota
	ModeSSA
)

const (
	ActionName = "deploy"
)

// Action deploys the resources that are included in the ReconciliationRequest using
// the same create or patch machinery implemented as part of deploy.DeployManifestsFromPath.
type Action struct {
	fieldOwner string
	deployMode Mode
}

type ActionOpts func(*Action)

func WithFieldOwner(value string) ActionOpts {
	return func(action *Action) {
		action.fieldOwner = value
	}
}
func WithMode(value Mode) ActionOpts {
	return func(action *Action) {
		action.deployMode = value
	}
}

func (r *Action) Execute(ctx context.Context, rr *odhTypes.ReconciliationRequest) error {
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

		old, err := r.lookup(ctx, rr.Client, obj)
		if err != nil {
			return fmt.Errorf("failed to lookup object %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
		}

		// if the resource is found and not managed by the operator, remove it from
		// the deployed resources
		if old != nil && old.GetAnnotations()[annotations.ManagedByODHOperator] == "false" {
			continue
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
			case ModePatch:
				err = r.patch(ctx, rr.Client, obj, old)
			case ModeSSA:
				err = r.apply(ctx, rr.Client, obj, old)
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

func (r *Action) lookup(ctx context.Context, c *odhClient.Client, obj unstructured.Unstructured) (*unstructured.Unstructured, error) {
	found := unstructured.Unstructured{}
	found.SetGroupVersionKind(obj.GroupVersionKind())

	// TODO: use PartialObjectMetadata for resources where it make sense
	err := c.Get(ctx, client.ObjectKeyFromObject(&obj), &found)
	switch {
	case err != nil && !k8serr.IsNotFound(err):
		return nil, err
	case err != nil && k8serr.IsNotFound(err):
		return nil, nil
	default:
		return &found, nil
	}
}

func (r *Action) patch(ctx context.Context, c *odhClient.Client, obj unstructured.Unstructured, old *unstructured.Unstructured) error {
	switch obj.GroupVersionKind() {
	case gvk.Deployment:
		// For deployments, we allow the user to change some parameters, such as
		// container resources and replicas except:
		// - If the resource does not exist (the resource must be created)
		// - If the resource is forcefully marked as managed by the operator via
		//   annotations (i.e. to bring it back to the default values)
		if old == nil {
			break
		}
		if old.GetAnnotations()[annotations.ManagedByODHOperator] == "true" {
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

	if old == nil {
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
			old,
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

func (r *Action) apply(ctx context.Context, c *odhClient.Client, obj unstructured.Unstructured, old *unstructured.Unstructured) error {
	switch obj.GroupVersionKind() {
	case gvk.Deployment:
		// For deployments, we allow the user to change some parameters, such as
		// container resources and replicas except:
		// - If the resource does not exist (the resource must be created)
		// - If the resource is forcefully marked as managed by the operator via
		//   annotations (i.e. to bring it back to the default values)
		if old == nil {
			break
		}
		if old.GetAnnotations()[annotations.ManagedByODHOperator] == "true" {
			break
		}

		// To preserve backward compatibility with the current model, fields are being
		// merged from an existing Deployment (if it exists) to the rendered manifest,
		// hence the current value is preserved [1].
		//
		// Ideally deployed resources should be configured only via the platform API
		//
		// [1] https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts
		if err := MergeDeployments(old, &obj); err != nil {
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

func New(opts ...ActionOpts) *Action {
	action := Action{
		deployMode: ModeSSA,
	}

	for _, opt := range opts {
		opt(&action)
	}

	return &action
}
