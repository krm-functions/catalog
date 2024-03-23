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
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/krm-functions/catalog/pkg/api"
	"github.com/krm-functions/catalog/pkg/helm"
	t "github.com/krm-functions/catalog/pkg/helmspecs"
	"github.com/krm-functions/catalog/pkg/semver"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
)

var upgradesEvaluated, upgradesDone, upgradesAvailable int

// evaluateChartVersion looks up versions and find a possible upgrade that fulfills upgradeConstraint
func evaluateChartVersion(chart t.HelmChartArgs, upgradeConstraint string, username, password *string) (*t.HelmChartArgs, error) {
	upgradesEvaluated++
	if upgradeConstraint == "" {
		upgradeConstraint = "*"
	}
	search, err := helm.SearchRepo(chart, username, password)
	if err != nil {
		return nil, err
	}
	search = helm.FilterByChartName(search, chart)
	versions := helm.ToList(search)
	newVersion, err := semver.Upgrade(versions, upgradeConstraint)
	if err != nil {
		return nil, err
	}

	newChart := chart
	newChart.Version = newVersion
	return &newChart, nil
}

// handleNewVersion applies new version to chart spec according to upgradeConstraint
func handleNewVersion(newChart, curr t.HelmChartArgs, kubeObject *fn.KubeObject, idx int, upgradeConstraint string) (*t.HelmChartArgs, string, error) {
	upgraded := curr
	var info string

	tmpDir, err := os.MkdirTemp("", "chart-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(tmpDir)

	if newChart.Version != curr.Version {
		upgradesAvailable++
		anno := curr.Repo + "/" + curr.Name + ":" + newChart.Version
		if Config.AnnotateOnUpgradeAvailable {
			if idx >= 0 {
				err := kubeObject.SetAnnotation(api.HelmResourceAnnotationUpgradeAvailable+"."+strconv.FormatInt(int64(idx), 10), anno)
				if err != nil {
					return nil, "", err
				}
			} else {
				err := kubeObject.SetAnnotation(api.HelmResourceAnnotationUpgradeAvailable, anno)
				if err != nil {
					return nil, "", err
				}
			}
		}
		if Config.UpgradeOnUpgradeAvailable {
			upgradesDone++
			upgraded.Version = newChart.Version
		}
		if Config.AnnotateSumOnUpgradeAvailable {
			_, chartSum, err := helm.PullChart(newChart, tmpDir, nil, nil)
			if err != nil {
				return nil, "", err
			}
			if idx >= 0 {
				err = kubeObject.SetAnnotation(api.HelmResourceAnnotationUpgradeShaSum+"."+strconv.FormatInt(int64(idx), 10), formatShaSum(chartSum))
				if err != nil {
					return nil, "", err
				}
			} else {
				err = kubeObject.SetAnnotation(api.HelmResourceAnnotationUpgradeShaSum, formatShaSum(chartSum))
				if err != nil {
					return nil, "", err
				}
			}
		}
		upgradedJSON, _ := json.Marshal(upgraded)
		currJSON, _ := json.Marshal(curr)
		distance, err := semver.Diff(curr.Version, upgraded.Version)
		if err != nil {
			return nil, "", err
		}
		info = fmt.Sprintf("{\"current\": %s, \"upgraded\": %s, \"constraint\": %q, \"semverDistance\": %q}\n", string(currJSON), string(upgradedJSON), upgradeConstraint, distance)
	} else if Config.AnnotateCurrentSum && kubeObject.GetAnnotation(api.HelmResourceAnnotationShaSum) == "" {
		_, chartSum, err := helm.PullChart(curr, tmpDir, nil, nil)
		if err != nil {
			return nil, "", err
		}
		err = kubeObject.SetAnnotation(api.HelmResourceAnnotationShaSum, formatShaSum(chartSum))
		if err != nil {
			return nil, "", err
		}
	}
	return &upgraded, info, nil
}

func Run(rl *fn.ResourceList) (bool, error) {
	cfg := rl.FunctionConfig
	parseConfig(cfg)
	results := &rl.Results

	for _, kubeObject := range rl.Items {
		if kubeObject.IsGVK("fn.kpt.dev", "", "RenderHelmChart") || kubeObject.IsGVK(api.HelmResourceAPI, "", "RenderHelmChart") {
			upgradeConstraint := kubeObject.GetAnnotation(api.HelmResourceAnnotationUpgradeConstraint)

			y := kubeObject.String()
			spec, err := t.ParseKptSpec([]byte(y))
			if err != nil {
				return false, err
			}
			for idx := range spec.Charts {
				helmChart := &spec.Charts[idx]
				var newVersion, upgraded *t.HelmChartArgs
				var info string
				var uname, pword *string
				if helmChart.Args.Auth != nil {
					uname, pword, err = helm.LookupAuthSecret(helmChart.Args.Auth.Name, helmChart.Args.Auth.Namespace, rl)
					if err != nil {
						return false, err
					}
				}
				newVersion, err = evaluateChartVersion(helmChart.Args, upgradeConstraint, uname, pword)
				if err != nil {
					return false, err
				}
				upgraded, info, err = handleNewVersion(*newVersion, helmChart.Args, kubeObject, idx, upgradeConstraint)
				if err != nil {
					return false, err
				}
				helmChart.Args.Version = upgraded.Version
				*results = append(*results, fn.ConfigObjectResult(info, kubeObject, fn.Info))
			}
			err = kubeObject.SetNestedField(spec.Charts, "helmCharts")
			if err != nil {
				return false, err
			}
		} else if kubeObject.IsGVK("argoproj.io", "", "Application") {
			upgradeConstraint := kubeObject.GetAnnotation(api.HelmResourceAnnotationUpgradeConstraint)

			y := kubeObject.String()
			app, err := t.ParseArgoCDSpec([]byte(y))
			if err != nil {
				return false, err
			}
			chartArgs := app.Spec.Source.ToKptSpec()
			newVersion, err := evaluateChartVersion(chartArgs, upgradeConstraint, nil, nil) // FIXME private repo not supported with Argo apps
			if err != nil {
				return false, err
			}
			upgraded, info, err := handleNewVersion(*newVersion, chartArgs, kubeObject, -1, upgradeConstraint)
			if err != nil {
				return false, err
			}
			*results = append(*results, fn.ConfigObjectResult(info, kubeObject, fn.Info))
			err = kubeObject.SetNestedField(upgraded.Version, "spec", "source", "targetRevision")
			if err != nil {
				return false, err
			}
		}
	}

	*results = append(*results, fn.GeneralResult(fmt.Sprintf("{\"upgradesEvaluated\": %d, \"upgradesDone\": %d, \"upgradesAvailable\": %d, \"upgradesSkipped\": %d}\n", upgradesEvaluated, upgradesDone, upgradesAvailable, upgradesAvailable-upgradesDone), fn.Info))
	return true, nil
}

func formatShaSum(sum string) string {
	return "sha256:" + sum
}

func main() {
	if err := fn.AsMain(fn.ResourceListProcessorFunc(Run)); err != nil {
		os.Exit(1)
	}
}
