package render

import (
	"context"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/manifests/kustomize"
)

const (
	ActionName = "render-manifests"
)

type Action struct {
	keOpts []kustomize.EngineOptsFn
	ke     *kustomize.Engine
}

type ActionOpts func(*Action)

func WithManifestsOptions(values ...kustomize.EngineOptsFn) ActionOpts {
	return func(action *Action) {
		action.keOpts = append(action.keOpts, values...)
	}
}

func (r *Action) Execute(_ context.Context, rr *types.ReconciliationRequest) error {
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

func New(opts ...ActionOpts) *Action {
	action := Action{}

	for _, opt := range opts {
		opt(&action)
	}

	action.ke = kustomize.NewEngine(action.keOpts...)

	return &action
}
