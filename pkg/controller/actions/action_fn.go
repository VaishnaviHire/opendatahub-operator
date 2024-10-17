package actions

import (
	"context"
	"reflect"
	"runtime"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

type ActionFn func(ctx context.Context, rr *types.ReconciliationRequest) error

type WrapperAction struct {
	BaseAction
	fn ActionFn
}

func (r *WrapperAction) Execute(ctx context.Context, rr *types.ReconciliationRequest) error {
	return r.fn(ctx, rr)
}

func NewActionFn(ctx context.Context, fn ActionFn) *WrapperAction {
	name := runtime.FuncForPC(reflect.ValueOf(fn).Pointer()).Name()

	action := WrapperAction{
		BaseAction: BaseAction{
			Log: log.FromContext(ctx).WithName(ActionGroup).WithName(name),
		},
		fn: fn,
	}

	return &action
}
