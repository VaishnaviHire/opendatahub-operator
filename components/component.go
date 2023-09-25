package components

import (
	dsci "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//	"k8s.io/apimachinery/pkg/runtime"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
)

type Component struct {
	// Set to one of the following values:
	//
	// - "Managed" : the operator is actively managing the component and trying to keep it active.
	//               It will only upgrade the component if it is safe to do so
	//
	// - "Removed" : the operator is actively managing the component and will not install it,
	//               or if it is installed, the operator will try to remove it
	//
	// +kubebuilder:validation:Enum=Managed;Removed
	ManagementState operatorv1.ManagementState `json:"managementState,omitempty"`
	// Add any other common fields across components below

	// Add developer fields
	// +optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=2
	DevFlags DevFlags `json:"devFlags,omitempty"`
}

func (c *Component) GetManagementState() operatorv1.ManagementState {
	return c.ManagementState
}

// DevFlags defines list of fields that can be used by developers to test customizations. This is not recommended
// to be used in production environment.
type DevFlags struct {
	// List of custom manifests for the given component
	// +optional
	Manifests []ManifestsConfig `json:"manifests,omitempty"`
}

type ManifestsConfig struct {
	// uri is the URI point to a git repo with tag/branch. e.g  https://github.com/org/repo/tarball/<tag/branch>
	// +optional
	// +kubebuilder:default:=""
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=1
	URI string `json:"uri,omitempty"`

	// contextDir is the relative path to the folder containing manifests in a repository
	// +optional
	// +kubebuilder:default:=""
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=2
	ContextDir string `json:"contextDir,omitempty"`

	// sourcePath is the subpath within contextDir where kustomize builds start. Examples include any sub-folder or path: `base`, `overlays/dev`, `default`, `odh` etc
	// +optional
	// +kubebuilder:default:=""
	// +operator-sdk:csv:customresourcedefinitions:type=spec,order=3
	SourcePath string `json:"sourcePath,omitempty"`
}

type ComponentInterface interface {
	ReconcileComponent(cli client.Client, owner metav1.Object, DSCISpec *dsci.DSCInitializationSpec) error
	GetComponentName() string
	GetManagementState() operatorv1.ManagementState
	GetComponentDevFlags() DevFlags
	SetImageParamsMap(imageMap map[string]string) map[string]string
	OverrideManifests(platform string) error
	UpdatePrometheusConfig(cli client.Client, enable bool, component string) error
}

func (c *Component) UpdatePrometheusConfig(cli client.Client, enable bool, component string) error {
	prometheusconfigPath := filepath.Join("/opt/manifests", "monitoring", "prometheus", "apps", "prometheus-configs.yaml")
	// create a struct to mock poremtheus.yml
	type ConfigMap struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
		Metadata   struct {
			Name string `yaml:"name"`
		} `yaml:"metadata"`
		Data struct {
			PrometheusYML string `yaml:"prometheus.yml"`
		} `yaml:"data"`
	}

	var configMap ConfigMap
	// read prometheus.yml from local disk /opt/mainfests&/monitoring/
	yamlData, err := os.ReadFile(prometheusconfigPath)
	if err != nil {
		return err
	}

	if err := yaml.Unmarshal([]byte(yamlData), &configMap); err != nil {
		panic(err)
	}
	// to add component rules when it is not there yet
	if enable {
		// Check if the rule already exists in rule_files
		ruleExists := strings.Contains(configMap.Data.PrometheusYML, component+"*.rules")

		// Add the new rule only if it doesn't exist
		if !ruleExists {
			configMap.Data.PrometheusYML += "\n		" + component + "*.rules"
		}
	} else { // to remove component rules if it is there already
		configMap.Data.PrometheusYML = strings.ReplaceAll(configMap.Data.PrometheusYML, "- "+component+"*.rules\n", "")
	}

	newyamlData, err := yaml.Marshal(&configMap)
	if err != nil {
		panic(err)
	}
	// Write the modified content back to the file
	err = os.WriteFile(prometheusconfigPath, newyamlData, 0)
	if err != nil {
		return err
	}
	return nil
}
