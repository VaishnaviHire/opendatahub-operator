package reconciler

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions/fn"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

type forInput struct {
	object  client.Object
	options []builder.ForOption
}

type watchInput struct {
	object       client.Object
	eventHandler handler.EventHandler
	options      []builder.WatchesOption
}

type ComponentReconcilerBuilder[T types.ResourceObject] struct {
	mgr           ctrl.Manager
	input         forInput
	watches       []watchInput
	predicates    []predicate.Predicate
	componentName string
	actions       []actions.Action
	finalizers    []actions.Action
}

func ComponentReconcilerFor[T types.ResourceObject](mgr ctrl.Manager, object client.Object, opts ...builder.ForOption) *ComponentReconcilerBuilder[T] {
	crb := ComponentReconcilerBuilder[T]{
		mgr: mgr,
		input: forInput{
			object:  object,
			options: slices.Clone(opts),
		},
	}

	return &crb
}

func (b *ComponentReconcilerBuilder[T]) WithComponentName(componentName string) *ComponentReconcilerBuilder[T] {
	b.componentName = componentName
	return b
}

func (b *ComponentReconcilerBuilder[T]) WithAction(value actions.Action) *ComponentReconcilerBuilder[T] {
	b.actions = append(b.actions, value)
	return b
}

func (b *ComponentReconcilerBuilder[T]) WithActionFn(value fn.Fn) *ComponentReconcilerBuilder[T] {
	b.actions = append(b.actions, fn.New(value))
	return b
}

func (b *ComponentReconcilerBuilder[T]) WithFinalizer(value actions.Action) *ComponentReconcilerBuilder[T] {
	b.finalizers = append(b.finalizers, value)
	return b
}

func (b *ComponentReconcilerBuilder[T]) WithFinalizerf(value fn.Fn) *ComponentReconcilerBuilder[T] {
	b.finalizers = append(b.finalizers, fn.New(value))
	return b
}

func (b *ComponentReconcilerBuilder[T]) Watches(object client.Object, eventHandler handler.EventHandler, opts ...builder.WatchesOption) *ComponentReconcilerBuilder[T] {
	b.watches = append(b.watches, watchInput{
		object:       object,
		eventHandler: eventHandler,
		options:      slices.Clone(opts),
	})

	return b
}

func (b *ComponentReconcilerBuilder[T]) WatchesM(object client.Object, fn handler.MapFunc, opts ...builder.WatchesOption) *ComponentReconcilerBuilder[T] {
	b.watches = append(b.watches, watchInput{
		object:       object,
		eventHandler: handler.EnqueueRequestsFromMapFunc(fn),
		options:      slices.Clone(opts),
	})

	return b
}

func (b *ComponentReconcilerBuilder[T]) WithEventFilter(p predicate.Predicate) *ComponentReconcilerBuilder[T] {
	b.predicates = append(b.predicates, p)
	return b
}

func (b *ComponentReconcilerBuilder[T]) Build(ctx context.Context) (*ComponentReconciler[T], error) {
	name := b.componentName
	if name == "" {
		name = b.input.object.GetObjectKind().GroupVersionKind().Kind
		name = strings.ToLower(name)
	}

	r, err := NewComponentReconciler[T](ctx, b.mgr, name)
	if err != nil {
		return nil, fmt.Errorf("failed to create reconciler for component %s: %w", b.componentName, err)
	}

	c := ctrl.NewControllerManagedBy(b.mgr)
	c = c.For(b.input.object, b.input.options...)

	for i := range b.watches {
		c = c.Watches(b.watches[i].object, b.watches[i].eventHandler, b.watches[i].options...)
	}
	for i := range b.predicates {
		c = c.WithEventFilter(b.predicates[i])
	}

	for i := range b.actions {
		r.AddAction(b.actions[i])
	}
	for i := range b.finalizers {
		r.AddFinalizer(b.finalizers[i])
	}

	return r, c.Complete(r)
}
