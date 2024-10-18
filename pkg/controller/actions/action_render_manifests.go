package actions

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/manifests/kustomize"
)

const (
	RenderManifestsActionName = "render-manifests"
)

type RenderManifestsAction struct {
	BaseAction

	keOpts []kustomize.EngineOptsFn
	ke     *kustomize.Engine
}

type RenderManifestsActionOpts func(*RenderManifestsAction)

func WithRenderManifestsOptions(values ...kustomize.EngineOptsFn) RenderManifestsActionOpts {
	return func(action *RenderManifestsAction) {
		action.keOpts = append(action.keOpts, values...)
	}
}

func (r *RenderManifestsAction) Execute(ctx context.Context, rr *types.ReconciliationRequest) error {
	for i := range rr.Manifests {
		opts := make([]kustomize.RenderOptsFn, 0, len(rr.Manifests[i].RenderOpts)+3)
		opts = append(opts, kustomize.WithNamespace(rr.DSCI.Spec.ApplicationsNamespace))
		opts = append(opts, rr.Manifests[i].RenderOpts...)

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

func NewRenderManifestsAction(ctx context.Context, opts ...RenderManifestsActionOpts) *RenderManifestsAction {
	action := RenderManifestsAction{
		BaseAction: BaseAction{
			Log: log.FromContext(ctx).WithName(ActionGroup).WithName(RenderManifestsActionName),
		},
	}

	for _, opt := range opts {
		opt(&action)
	}

	action.ke = kustomize.NewEngine(action.keOpts...)

	return &action
}
