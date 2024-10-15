package components

import (
	"context"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Action interface {
	Execute(ctx context.Context, rr *ReconciliationRequest) error
}

type BaseAction struct {
	Log logr.Logger
}

type InitializeAction struct {
	BaseAction
}

type SupportDevFlagsAction struct {
	BaseAction
}

type CleanupOAuthClientAction struct {
	BaseAction
}

type DeployComponentAction struct {
	BaseAction
}

type UpdateStatusAction struct {
	BaseAction
}

type DeleteResourcesAction struct {
	BaseAction
	Types  []client.Object
	Labels map[string]string
}
