package main

import (
	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
)

type fnConfig struct {
	AnnotateOnUpgradeAvailable    bool `json:"annotateOnUpgradeAvailable,omitempty" yaml:"annotateOnUpgradeAvailable,omitempty"`
	AnnotateSumOnUpgradeAvailable bool `json:"annotateSumOnUpgradeAvailable,omitempty" yaml:"annotateSumOnUpgradeAvailable,omitempty"`
	UpgradeOnUpgradeAvailable     bool `json:"upgradeOnUpgradeAvailable,omitempty" yaml:"upgradeOnUpgradeAvailable,omitempty"`
	AnnotateCurrentSum            bool `json:"annotateCurrentSum,omitempty" yaml:"annotateCurrentSum,omitempty"`
}

var Config fnConfig

func parseConfig(configmap *fn.KubeObject) {
	if val, found, _ := configmap.NestedBool("data", "annotateOnUpgradeAvailable"); found {
		Config.AnnotateOnUpgradeAvailable = val
	}
	if val, found, _ := configmap.NestedBool("data", "annotateSumOnUpgradeAvailable"); found {
		Config.AnnotateSumOnUpgradeAvailable = val
	}
	if val, found, _ := configmap.NestedBool("data", "upgradeOnUpgradeAvailable"); found {
		Config.UpgradeOnUpgradeAvailable = val
	} else {
		Config.UpgradeOnUpgradeAvailable = true // Not found, default to upgrade
	}
	if val, found, _ := configmap.NestedBool("data", "annotateCurrentSum"); found {
		Config.AnnotateCurrentSum = val
	}
}
