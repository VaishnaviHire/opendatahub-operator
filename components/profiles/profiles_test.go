package profiles_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	dsc "github.com/opendatahub-io/opendatahub-operator/apis/datasciencecluster/v1alpha1"
	"github.com/opendatahub-io/opendatahub-operator/components/dashboard"
	"github.com/opendatahub-io/opendatahub-operator/components/profiles"
)

var _ = Describe("Profiles", func() {
	var baseServingProfile, baseTrainingProfile, baseWorkbenchesProfile, baseFullProfile, baseEmptyProfile *dsc.DataScienceCluster
	var servingPlusProfile, trainingPlusProfile, workbenchesPlusProfile, fullPlusProfile *dsc.DataScienceCluster

	Context("Default profiles without overrides", func() {
		BeforeEach(func() {
			baseServingProfile = &dsc.DataScienceCluster{
				Spec: dsc.DataScienceClusterSpec{
					Profile: dsc.ProfileServing,
				},
			}
			baseTrainingProfile = &dsc.DataScienceCluster{
				Spec: dsc.DataScienceClusterSpec{
					Profile: dsc.ProfileTraining,
				},
			}
			baseWorkbenchesProfile = &dsc.DataScienceCluster{
				Spec: dsc.DataScienceClusterSpec{
					Profile: dsc.ProfileWorkbench,
				},
			}
			baseFullProfile = &dsc.DataScienceCluster{
				Spec: dsc.DataScienceClusterSpec{
					Profile: dsc.ProfileFull,
				},
			}
			baseEmptyProfile = &dsc.DataScienceCluster{}
		})

		It("Serving profile should enable only the serving components", func() {
			plan := profiles.CreateReconciliationPlan(baseServingProfile)

			Expect(plan.Components[profiles.ServingComponent]).To(BeTrue())
			Expect(plan.Components[profiles.TrainingComponent]).To(BeFalse())
			Expect(plan.Components[profiles.WorkbenchesComponent]).To(BeFalse())
			Expect(plan.Components[dashboard.ComponentName]).To(BeTrue())
		})
		It("Training profile should enable only the training components", func() {
			plan := profiles.CreateReconciliationPlan(baseTrainingProfile)

			Expect(plan.Components[profiles.ServingComponent]).To(BeFalse())
			Expect(plan.Components[profiles.TrainingComponent]).To(BeTrue())
			Expect(plan.Components[profiles.WorkbenchesComponent]).To(BeFalse())
			Expect(plan.Components[dashboard.ComponentName]).To(BeTrue())
		})
		It("Workbenches profile should enable only the workbench components", func() {
			plan := profiles.CreateReconciliationPlan(baseWorkbenchesProfile)

			Expect(plan.Components[profiles.ServingComponent]).To(BeFalse())
			Expect(plan.Components[profiles.TrainingComponent]).To(BeFalse())
			Expect(plan.Components[profiles.WorkbenchesComponent]).To(BeTrue())
			Expect(plan.Components[dashboard.ComponentName]).To(BeTrue())
		})
		It("Full profile should enable all components", func() {
			plan := profiles.CreateReconciliationPlan(baseFullProfile)

			Expect(plan.Components[profiles.ServingComponent]).To(BeTrue())
			Expect(plan.Components[profiles.TrainingComponent]).To(BeTrue())
			Expect(plan.Components[profiles.WorkbenchesComponent]).To(BeTrue())
			Expect(plan.Components[dashboard.ComponentName]).To(BeTrue())
		})
		It("Empty profile defaults to Full and should enable all components", func() {
			plan := profiles.CreateReconciliationPlan(baseEmptyProfile)

			Expect(plan.Components[profiles.ServingComponent]).To(BeTrue())
			Expect(plan.Components[profiles.TrainingComponent]).To(BeTrue())
			Expect(plan.Components[profiles.WorkbenchesComponent]).To(BeTrue())
			Expect(plan.Components[dashboard.ComponentName]).To(BeTrue())
		})
	})

	Context("Profiles with overrides", func() {
		BeforeEach(func() {
			t := true
			f := false
			servingPlusProfile = &dsc.DataScienceCluster{
				Spec: dsc.DataScienceClusterSpec{
					Profile: dsc.ProfileServing,
					Components: dsc.Components{
						Serving: dsc.Serving{
							Component: dsc.Component{Enabled: &f},
						},
						Training: dsc.Training{
							Component: dsc.Component{Enabled: &t},
						},
						Workbenches: dsc.Workbenches{
							Component: dsc.Component{Enabled: &t},
						},
						Dashboard: dsc.Dashboard{
							Component: dsc.Component{Enabled: &f},
						},
					},
				},
			}
			trainingPlusProfile = &dsc.DataScienceCluster{
				Spec: dsc.DataScienceClusterSpec{
					Profile: dsc.ProfileTraining,
					Components: dsc.Components{
						Serving: dsc.Serving{
							Component: dsc.Component{Enabled: &t},
						},
						Training: dsc.Training{
							Component: dsc.Component{Enabled: &f},
						},
						Workbenches: dsc.Workbenches{
							Component: dsc.Component{Enabled: &t},
						},
						Dashboard: dsc.Dashboard{
							Component: dsc.Component{Enabled: &f},
						},
					},
				},
			}
			workbenchesPlusProfile = &dsc.DataScienceCluster{
				Spec: dsc.DataScienceClusterSpec{
					Profile: dsc.ProfileWorkbench,
					Components: dsc.Components{
						Serving: dsc.Serving{
							Component: dsc.Component{Enabled: &t},
						},
						Training: dsc.Training{
							Component: dsc.Component{Enabled: &t},
						},
						Workbenches: dsc.Workbenches{
							Component: dsc.Component{Enabled: &f},
						},
						Dashboard: dsc.Dashboard{
							Component: dsc.Component{Enabled: &f},
						},
					},
				},
			}
			fullPlusProfile = &dsc.DataScienceCluster{
				Spec: dsc.DataScienceClusterSpec{
					Profile: dsc.ProfileFull,
					Components: dsc.Components{
						Serving: dsc.Serving{
							Component: dsc.Component{Enabled: &f},
						},
						Training: dsc.Training{
							Component: dsc.Component{Enabled: &f},
						},
						Workbenches: dsc.Workbenches{
							Component: dsc.Component{Enabled: &f},
						},
						Dashboard: dsc.Dashboard{
							Component: dsc.Component{Enabled: &f},
						},
					},
				},
			}
		})

		It("Serving profile with opposite overrides", func() {
			plan := profiles.CreateReconciliationPlan(servingPlusProfile)

			Expect(plan.Components[profiles.ServingComponent]).To(BeFalse())
			Expect(plan.Components[profiles.TrainingComponent]).To(BeTrue())
			Expect(plan.Components[profiles.WorkbenchesComponent]).To(BeTrue())
			Expect(plan.Components[dashboard.ComponentName]).To(BeFalse())
		})
		It("Training profile with opposite overrides", func() {
			plan := profiles.CreateReconciliationPlan(trainingPlusProfile)

			Expect(plan.Components[profiles.ServingComponent]).To(BeTrue())
			Expect(plan.Components[profiles.TrainingComponent]).To(BeFalse())
			Expect(plan.Components[profiles.WorkbenchesComponent]).To(BeTrue())
			Expect(plan.Components[dashboard.ComponentName]).To(BeFalse())
		})
		It("Workbench profile with opposite overrides", func() {
			plan := profiles.CreateReconciliationPlan(workbenchesPlusProfile)

			Expect(plan.Components[profiles.ServingComponent]).To(BeTrue())
			Expect(plan.Components[profiles.TrainingComponent]).To(BeTrue())
			Expect(plan.Components[profiles.WorkbenchesComponent]).To(BeFalse())
			Expect(plan.Components[dashboard.ComponentName]).To(BeFalse())
		})
		It("Full profile with opposite overrides", func() {
			plan := profiles.CreateReconciliationPlan(fullPlusProfile)

			Expect(plan.Components[profiles.ServingComponent]).To(BeFalse())
			Expect(plan.Components[profiles.TrainingComponent]).To(BeFalse())
			Expect(plan.Components[profiles.WorkbenchesComponent]).To(BeFalse())
			Expect(plan.Components[dashboard.ComponentName]).To(BeFalse())
		})
	})

})
