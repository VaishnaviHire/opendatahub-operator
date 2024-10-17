package actions_test

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/client"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
)

func NewFakeClient(ctx context.Context, objs ...ctrlClient.Object) (*client.Client, error) {
	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(appsv1.AddToScheme(scheme))

	fakeMapper := meta.NewDefaultRESTMapper(scheme.PreferredVersionAllGroups())
	for gvk := range scheme.AllKnownTypes() {
		fakeMapper.Add(gvk, meta.RESTScopeNamespace)
	}

	return client.New(
		ctx,
		nil,
		fake.NewClientBuilder().
			WithScheme(scheme).
			WithRESTMapper(fakeMapper).
			WithObjects(objs...).
			Build(),
	)
}

func ExtractStatusCondition(conditionType string) func(in types.ResourceObject) metav1.Condition {
	return func(in types.ResourceObject) metav1.Condition {
		c := meta.FindStatusCondition(in.GetStatus().Conditions, conditionType)
		if c == nil {
			return metav1.Condition{}
		}

		return *c
	}
}
