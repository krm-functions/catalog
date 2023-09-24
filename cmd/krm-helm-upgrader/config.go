// Copyright 2023 Michael Vittrup Larsen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	if val, found, err := configmap.NestedString("data", "annotateOnUpgradeAvailable"); err==nil && found {
		Config.AnnotateOnUpgradeAvailable = (val=="true" || val=="1")
	}
	if val, found, err := configmap.NestedString("data", "annotateSumOnUpgradeAvailable"); err==nil && found {
		Config.AnnotateSumOnUpgradeAvailable = (val=="true" || val=="1")
	}
	if val, found, err := configmap.NestedString("data", "upgradeOnUpgradeAvailable"); err==nil && found {
		Config.UpgradeOnUpgradeAvailable = (val=="true" || val=="1")
	} else {
		Config.UpgradeOnUpgradeAvailable = true // Not found, default to upgrade
	}
	if val, found, err := configmap.NestedString("data", "annotateCurrentSum"); err==nil && found {
		Config.AnnotateCurrentSum = (val=="true" || val=="1")
	}
}
