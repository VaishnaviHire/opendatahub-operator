package profiles

import (
	dsc "github.com/opendatahub-io/opendatahub-operator/apis/datasciencecluster/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/components/dashboard"
)

type ReconciliationPlan struct {
	Components map[string]bool
}

type ProfileConfig struct {
	ComponentDefaults map[string]bool
}

// TODO: once the component packages are created, move them to the component's package
const (
	ServingComponent     = "serving"
	TrainingComponent    = "training"
	WorkbenchesComponent = "workbenches"
)

var profileConfigs = map[string]ProfileConfig{
	dsc.ProfileServing: {
		ComponentDefaults: map[string]bool{
			ServingComponent:        true,
			TrainingComponent:       false,
			WorkbenchesComponent:    false,
			dashboard.ComponentName: true,
		},
	},
	dsc.ProfileTraining: {
		ComponentDefaults: map[string]bool{
			ServingComponent:        false,
			TrainingComponent:       true,
			WorkbenchesComponent:    false,
			dashboard.ComponentName: true,
		},
	},
	dsc.ProfileWorkbench: {
		ComponentDefaults: map[string]bool{
			ServingComponent:        false,
			TrainingComponent:       false,
			WorkbenchesComponent:    true,
			dashboard.ComponentName: true,
		},
	},
	dsc.ProfileFull: {
		ComponentDefaults: map[string]bool{
			ServingComponent:        true,
			TrainingComponent:       true,
			WorkbenchesComponent:    true,
			dashboard.ComponentName: true,
		},
	},
	// Add more profiles and their component defaults as needed
}

func CreateReconciliationPlan(instance *dsc.DataScienceCluster) *ReconciliationPlan {
	plan := &ReconciliationPlan{
		Components: make(map[string]bool),
	}

	profile := instance.Spec.Profile // TODO: need to handle the case where the profile does not exist
	if profile == "" {
		profile = dsc.ProfileFull
	}

	populatePlan(profileConfigs[profile], plan, instance)
	// Similarly set other profiles
	return plan
}

func populatePlan(profiledefaults ProfileConfig, plan *ReconciliationPlan, instance *dsc.DataScienceCluster) {
	// serving is set to the default value, unless explicitly overriden
	plan.Components[ServingComponent] = profiledefaults.ComponentDefaults[ServingComponent]
	if instance.Spec.Components.Serving.Enabled != nil {
		plan.Components[ServingComponent] = *instance.Spec.Components.Serving.Enabled
	}
	// training is set to the default value, unless explicitly overriden
	plan.Components[TrainingComponent] = profiledefaults.ComponentDefaults[TrainingComponent]
	if instance.Spec.Components.Training.Enabled != nil {
		plan.Components[TrainingComponent] = *instance.Spec.Components.Training.Enabled
	}
	// workbenches is set to the default value, unless explicitly overriden
	plan.Components[WorkbenchesComponent] = profiledefaults.ComponentDefaults[WorkbenchesComponent]
	if instance.Spec.Components.Workbenches.Enabled != nil {
		plan.Components[WorkbenchesComponent] = *instance.Spec.Components.Workbenches.Enabled
	}
	// dashboard is set to the default value, unless explicitly overriden
	plan.Components[dashboard.ComponentName] = profiledefaults.ComponentDefaults[dashboard.ComponentName]
	if instance.Spec.Components.Dashboard.Enabled != nil {
		plan.Components[dashboard.ComponentName] = *instance.Spec.Components.Dashboard.Enabled
	}
}
