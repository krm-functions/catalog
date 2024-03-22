package api

const (
	HelmResourceAPI                         = "experimental.helm.sh"
	HelmResourceAnnotationShaSum            = HelmResourceAPI + "/chart-sum"
	HelmResourceAnnotationUpgradeAvailable  = HelmResourceAPI + "/upgrade-available"
	HelmResourceAnnotationUpgradeConstraint = HelmResourceAPI + "/upgrade-constraint"
	HelmResourceAnnotationUpgradeShaSum     = HelmResourceAPI + "/upgrade-chart-sum"
	HelmResourceAPIVersion                  = HelmResourceAPI + "/v1alpha1"
)
