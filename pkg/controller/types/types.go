package types

import (
	"path"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/opendatahub-operator/v2/apis/components"
	dscv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/datasciencecluster/v1"
	dsciv1 "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/cluster"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/manifests/kustomize"

	odhClient "github.com/opendatahub-io/opendatahub-operator/v2/pkg/controller/client"
)

type ResourceObject interface {
	client.Object
	components.WithStatus
}

type WithLogger interface {
	GetLogger() logr.Logger
}

type ManifestInfo struct {
	Path       string
	ContextDir string
	SourcePath string
	RenderOpts []kustomize.RenderOptsFn
}

func (mi *ManifestInfo) ManifestsPath() string {
	result := mi.Path

	if mi.ContextDir == "" {
		result = path.Join(result, mi.ContextDir)
	}

	if mi.SourcePath == "" {
		result = path.Join(result, mi.SourcePath)
	}

	return result
}

type ReconciliationRequest struct {
	*odhClient.Client
	Instance  client.Object
	DSC       *dscv1.DataScienceCluster
	DSCI      *dsciv1.DSCInitialization
	Platform  cluster.Platform
	Manifests []ManifestInfo
	Resources []unstructured.Unstructured
}
