package actions_test

import (
	"context"
	"path"
	"testing"

	"github.com/rs/xid"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/kustomize/kyaml/filesys"

	dscv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/datasciencecluster/v1"
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster/gvk"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/actions"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/types"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/manifests/kustomize"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/utils/test/matchers/jq"

	. "github.com/onsi/gomega"
)

const testRenderResourcesKustomization = `
apiVersion: kustomize.config.k8s.io/v1beta1
resources:
- test-resources-cm.yaml
- test-resources-deployment-managed.yaml
- test-resources-deployment-unmanaged.yaml
- test-resources-deployment-forced.yaml
`

const testRenderResourcesConfigMap = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  foo: bar
`

const testRenderResourcesManaged = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment-managed
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        resources:
            limits:
              memory: 200Mi
              cpu: 1
            requests:
              memory: 100Mi
              cpu: 100m
`

const testRenderResourcesUnmanaged = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment-unmanaged
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        resources:
            limits:
              memory: 200Mi
              cpu: 1
            requests:
              memory: 100Mi
              cpu: 100m
`
const testRenderResourcesForced = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deployment-forced
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
`

func TestRenderResourcesAction(t *testing.T) {
	g := NewWithT(t)

	ctx := context.Background()
	ns := xid.New().String()
	id := xid.New().String()
	fs := filesys.MakeFsInMemory()

	_ = fs.MkdirAll(path.Join(id, kustomize.DefaultKustomizationFilePath))
	_ = fs.WriteFile(path.Join(id, kustomize.DefaultKustomizationFileName), []byte(testRenderResourcesKustomization))
	_ = fs.WriteFile(path.Join(id, "test-resources-cm.yaml"), []byte(testRenderResourcesConfigMap))
	_ = fs.WriteFile(path.Join(id, "test-resources-deployment-managed.yaml"), []byte(testRenderResourcesManaged))
	_ = fs.WriteFile(path.Join(id, "test-resources-deployment-unmanaged.yaml"), []byte(testRenderResourcesUnmanaged))
	_ = fs.WriteFile(path.Join(id, "test-resources-deployment-forced.yaml"), []byte(testRenderResourcesForced))

	client, err := NewFakeClient(
		&appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gvk.Deployment.GroupVersion().String(),
				Kind:       gvk.Deployment.Kind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-managed",
				Namespace: ns,
				Labels:    map[string]string{},
			},
		},
		&appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gvk.Deployment.GroupVersion().String(),
				Kind:       gvk.Deployment.Kind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-unmanaged",
				Namespace: ns,
				Annotations: map[string]string{
					"opendatahub.io/managed": "false",
				},
			},
		},
		&appsv1.Deployment{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gvk.Deployment.GroupVersion().String(),
				Kind:       gvk.Deployment.Kind,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-deployment-forced",
				Namespace: ns,
				Annotations: map[string]string{
					"opendatahub.io/managed": "true",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To[int32](1),
			},
		},
	)

	g.Expect(err).ShouldNot(HaveOccurred())

	action := actions.NewRenderManifestsAction(
		ctx,
		actions.WithRenderManifestsOptions(
			kustomize.WithEngineFS(fs),
			kustomize.WithEngineRenderOpts(
				kustomize.WithAnnotations(map[string]string{
					"platform.opendatahub.io/release": "1.2.3",
					"platform.opendatahub.io/type":    "managed",
				}),
			),
		),
	)

	rr := types.ReconciliationRequest{
		Client:   client,
		Instance: nil,
		DSCI:     &dsciv1.DSCInitialization{Spec: dsciv1.DSCInitializationSpec{ApplicationsNamespace: ns}},
		DSC:      &dscv1.DataScienceCluster{},
		Platform: cluster.OpenDataHub,
		Manifests: []types.ManifestInfo{
			{
				Path: id,
				RenderOpts: []kustomize.RenderOptsFn{
					kustomize.WithLabel("component.opendatahub.io/name", "foo"),
					kustomize.WithLabel("platform.opendatahub.io/namespace", ns),
				},
			},
		},
	}

	err = action.Execute(ctx, &rr)

	g.Expect(err).ShouldNot(HaveOccurred())

	// common customizations
	g.Expect(rr.Resources).Should(And(
		HaveLen(3),
		HaveEach(And(
			jq.Match(`.metadata.namespace == "%s"`, ns),
			jq.Match(`.metadata.labels."component.opendatahub.io/name" == "%s"`, "foo"),
			jq.Match(`.metadata.labels."platform.opendatahub.io/namespace" == "%s"`, ns),
			jq.Match(`.metadata.annotations."platform.opendatahub.io/release" == "%s"`, "1.2.3"),
			jq.Match(`.metadata.annotations."platform.opendatahub.io/type" == "%s"`, "managed"),
		)),
	))

	// config map
	g.Expect(rr.Resources[0]).Should(And(
		jq.Match(`.metadata.name == "%s"`, "test-cm"),
	))

	// deployment managed
	g.Expect(rr.Resources[1]).Should(And(
		jq.Match(`.metadata.name == "%s"`, "test-deployment-managed"),
		jq.Match(`.spec.template.spec.containers[0] | has("resources") | not`),
	))

	// deployment forced
	g.Expect(rr.Resources[2]).Should(And(
		jq.Match(`.metadata.name == "%s"`, "test-deployment-forced"),
		jq.Match(`.spec.replicas == 3`),
	))
}
