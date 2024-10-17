package actions

import (
	"context"

	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/manifests/kustomize"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/metadata/annotations"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/plugins"
)

const (
	RenderManifestsActionName = "render-manifests"
)

type RenderManifestsAction struct {
	BaseAction

	keOpts          []kustomize.EngineOptsFn
	ke              *kustomize.Engine
	enableAllowList bool
}

type RenderManifestsActionOpts func(*RenderManifestsAction)

func WithRenderManifestsOptions(values ...kustomize.EngineOptsFn) RenderManifestsActionOpts {
	return func(action *RenderManifestsAction) {
		action.keOpts = append(action.keOpts, values...)
	}
}
func WithRenderManifestsAllowList(value bool) RenderManifestsActionOpts {
	return func(action *RenderManifestsAction) {
		action.enableAllowList = value
	}
}

func (r *RenderManifestsAction) Execute(ctx context.Context, rr *types.ReconciliationRequest) error {
	for i := range rr.Manifests {
		opts := make([]kustomize.RenderOptsFn, 0, len(rr.Manifests[i].RenderOpts)+3)
		opts = append(opts, kustomize.WithNamespace(rr.DSCI.Spec.ApplicationsNamespace))
		opts = append(opts, kustomize.WithFilter(filterUnmanged(ctx, rr)))
		opts = append(opts, rr.Manifests[i].RenderOpts...)

		if r.enableAllowList {
			opts = append(opts, kustomize.WithFilter(applyAllowList(ctx, rr)))
		}

		resources, err := r.ke.Render(
			rr.Manifests[i].ManifestsPath(),
			opts...,
		)

		if err != nil {
			return err
		}

		rr.Resources = append(rr.Resources, resources...)
	}

	return nil
}

func filterUnmanged(ctx context.Context, rr *types.ReconciliationRequest) kustomize.FilterFn {
	return func(nodes []*kyaml.RNode) ([]*kyaml.RNode, error) {
		result := make([]*kyaml.RNode, 0, len(nodes))

		for i := range nodes {
			// TODO we should use PartialObjectMetadata to only retrieve the resource
			//      metadata since we do not need anything more, but this requires the
			//      cache to be reconfigured according. Some resources should still be
			//      fully cached (i.e. Deployments)
			//      po := metav1.PartialObjectMetadata{}
			//      po.SetGroupVersionKind(obj.GroupVersionKind())
			u := kustomize.NodeToUnstructured(nodes[i])

			err := rr.Client.Get(ctx, client.ObjectKeyFromObject(&u), &u)
			if err != nil && !k8serr.IsNotFound(err) {
				return nodes, err
			}

			// if the resource is found and not managed by the operator, remove it from
			// the resources
			if err == nil && u.GetAnnotations()[annotations.ManagedByODHOperator] == "false" {
				continue
			}

			result = append(result, nodes[i])
		}

		return result, nil
	}
}

func applyAllowList(ctx context.Context, rr *types.ReconciliationRequest) kustomize.FilterFn {
	fieldsToClean := [][]string{
		{"spec", "template", "spec", "containers", "*", "resources"},
		{"spec", "replicas"},
	}

	return func(nodes []*kyaml.RNode) ([]*kyaml.RNode, error) {
		for i := range nodes {
			// TODO we should use PartialObjectMetadata to only retrieve the resource
			//      metadata since we do not need anything more, but this requires the
			//      cache to be reconfigured according. Some resources should still be
			//      fully cached (i.e. Deployments)
			//      po := metav1.PartialObjectMetadata{}
			//      po.SetGroupVersionKind(obj.GroupVersionKind())
			u := kustomize.NodeToUnstructured(nodes[i])

			if u.GroupVersionKind() != gvk.Deployment {
				continue
			}

			err := rr.Client.Get(ctx, client.ObjectKeyFromObject(&u), &u)
			if err != nil && !k8serr.IsNotFound(err) {
				return nodes, err
			}

			// If the resource does not exist, then don't apply the allow list
			// as the resource must be created
			if k8serr.IsNotFound(err) {
				continue
			}

			// If the resource is forcefully marked as managed by the operator, don't apply
			// the allow list as the user is explicitly forcing the resource to be managed
			// i.e. to restore default values
			if u.GetAnnotations()[annotations.ManagedByODHOperator] == "true" {
				continue
			}

			// To preserve backward compatibility with the current model, fields are being
			// removed, hence not included in the final PATCH. Ideally with should leverage
			// Server-Side Apply [1] and copy the values from the actual resource or even
			// better, make it possible to change operand's manifests only via the platform
			// APIs.
			//
			// [1] https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts
			for _, fields := range fieldsToClean {
				nodes[i], err = plugins.ClearField(nodes[i], fields)
				if err != nil {
					return nil, err
				}
			}
		}

		return nodes, nil
	}
}

func NewRenderManifestsAction(ctx context.Context, opts ...RenderManifestsActionOpts) *RenderManifestsAction {
	action := RenderManifestsAction{
		BaseAction: BaseAction{
			Log: log.FromContext(ctx).WithName(ActionGroup).WithName(RenderManifestsActionName),
		},
		enableAllowList: true,
	}

	for _, opt := range opts {
		opt(&action)
	}

	action.ke = kustomize.NewEngine(action.keOpts...)

	return &action
}
