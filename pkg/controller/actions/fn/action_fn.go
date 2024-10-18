package fn

import (
	"context"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

type Fn func(ctx context.Context, rr *types.ReconciliationRequest) error

type WrapperAction struct {
	fn Fn
}

func (r *WrapperAction) Execute(ctx context.Context, rr *types.ReconciliationRequest) error {
	return r.fn(ctx, rr)
}

func New(fn Fn) *WrapperAction {
	action := WrapperAction{
		fn: fn,
	}

	return &action
}
