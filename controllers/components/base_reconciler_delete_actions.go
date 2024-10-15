package components

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// TODO: Types and Labels becomes AND logic for selection?
// What if not set Types in the caller

// Execute deletes all resources if match the specified labels either as cluster-scope or from application namespace only.
func (r *DeleteResourcesAction) Execute(ctx context.Context, rr *ReconciliationRequest) error {
	for i := range r.Types {
		opts := make([]client.DeleteAllOfOption, 0)
		opts = append(opts, client.MatchingLabels(r.Labels))

		namespaced, err := rr.Client.IsObjectNamespaced(r.Types[i])
		if err != nil {
			return err
		}

		if namespaced {
			opts = append(opts, client.InNamespace(rr.DSCI.Spec.ApplicationsNamespace))
		}

		err = rr.Client.DeleteAllOf(ctx, r.Types[i], opts...)
		if err != nil {
			return err
		}
	}

	return nil
}
