// Package sremonitoring provides utility to reconcile SRE monitoring secret, only used in Managed/add-on cluster
package sremonitoring

import (
	"context"
	// "crypto/sha256"
	// b64 "encoding/base64"
	"path/filepath"

	dsci "github.com/opendatahub-io/opendatahub-operator/v2/apis/dscinitialization/v1"
	"github.com/opendatahub-io/opendatahub-operator/v2/components"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/common"
	"github.com/opendatahub-io/opendatahub-operator/v2/pkg/deploy"
	operatorv1 "github.com/openshift/api/operator/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ComponentName    = "monitoring"
	alertmanagerPath = deploy.DefaultManifestPath + "/" + ComponentName + "/alertmanager"
	prometheusPath   = deploy.DefaultManifestPath + "/" + ComponentName + "/prometheus"
)

type SREMonitoring struct {
	components.Component `json:""`
}

var imageParamMap = map[string]string{}

var _ components.ComponentInterface = (*SREMonitoring)(nil)

func (s *SREMonitoring) ReconcileComponent(cli client.Client, owner metav1.Object, dscispec *dsci.DSCInitializationSpec) error {
	monitoringEnabled := dscispec.Monitoring.ManagementState == operatorv1.Managed
	platform, err := deploy.GetPlatform(cli)
	if err != nil {
		return err
	}

	if monitoringEnabled {
		// update default rolebinding for monitoring namespace
		err := common.UpdatePodSecurityRolebinding(cli, []string{"redhat-ods-monitoring"}, dscispec.Monitoring.Namespace)
		if err != nil {
			return err
		}

		// TODO: cleanup selfmanaged before final code merge
		if platform == deploy.ManagedRhods || platform == deploy.SelfManagedRhods {
			// get email from secret
			addonODHSecret := &corev1.Secret{}
			err = cli.Get(context.TODO(), client.ObjectKey{Namespace: "redhat-ods-operator", Name: "addon-managed-odh-parameters"}, addonODHSecret)
			if err != nil {
				return err
			}
			smtpEmailSecret := addonODHSecret.Data["notification-email"]
			// replace smtpEmailSecret in alertmanager-configs.yaml
			if err = common.MatchLineInFile(filepath.Join(alertmanagerPath, "alertmanager-configs.yaml"),
				map[string]string{
					"- to:": "- to: " + string(smtpEmailSecret),
				},
			); err != nil {
				return err
			}

			// reconcile ConfigMap 'alertmanager'
			if err := deploy.DeployManifestsFromPath(cli, owner, alertmanagerPath, dscispec.Monitoring.Namespace, "alertmanager", monitoringEnabled); err != nil {
				return err
			}
			// get ConfigMap 'alertmanager' to create new hash
			alertManagerConfigMap := &corev1.ConfigMap{}
			err = cli.Get(context.TODO(), client.ObjectKey{
				Namespace: dscispec.Monitoring.Namespace,
				Name:      "alertmanager",
			}, alertManagerConfigMap)
			if err != nil {
				return err
			}

			alertmanagerData, err := common.GetMonitoringData(alertManagerConfigMap.Data["alertmanager.yml"])
			if err != nil {
				return err
			}
			// udpate annotation of Deployment 'prometheus'
			if err = common.MatchLineInFile(filepath.Join(prometheusPath, "base", "alertmanager-configs.yaml"),
				map[string]string{
					"alertmanager: ": "alertmanager: " + string(alertmanagerData),
				},
			); err != nil {
				return err
			}
			if err := deploy.DeployManifestsFromPath(cli, owner, prometheusPath+"/base", dscispec.Monitoring.Namespace, "prometheus", monitoringEnabled); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *SREMonitoring) SetImageParamsMap(imageMap map[string]string) map[string]string {
	imageParamMap = imageMap
	return imageParamMap
}

func (s *SREMonitoring) OverrideManifests(_ string) error {
	// noop
	return nil
}

func (s *SREMonitoring) GetComponentName() string {
	return ComponentName
}

func (s *SREMonitoring) DeepCopyInto(target *SREMonitoring) {
	*target = *s
	target.Component = s.Component
}
