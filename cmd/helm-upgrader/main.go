package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/michaelvl/helm-upgrader/pkg/helm"
	t "github.com/michaelvl/helm-upgrader/pkg/helmspecs"
	"github.com/michaelvl/helm-upgrader/pkg/semver"
	"github.com/michaelvl/helm-upgrader/pkg/version"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
)

const annotationUrl string = "experimental.helm.sh/"
const annotationUpgradeConstraint string = annotationUrl + "upgrade-constraint"
const annotationUpgradeAvailable string = annotationUrl + "upgrade-available"
const annotationShaSum string = annotationUrl + "chart-sum"
const annotationUpgradeShaSum string = annotationUrl + "upgrade-chart-sum"

var upgradesDone, upgradesAvailable int

func evaluateChartVersion(chart t.HelmChartArgs, upgradeConstraint string) (*t.HelmChartArgs, error) {
	if upgradeConstraint == "" {
		upgradeConstraint = "*"
	}
	search, err := helm.RepoSearch(chart)
	if err != nil {
		return nil, err
	}
	search = helm.FilterByChartName(search, chart)
	versions := helm.ToList(search)
	new_version, err := semver.Upgrade(versions, upgradeConstraint)
	if err != nil {
		return nil, err
	}

	new := chart
	new.Version = new_version
	return &new, nil
}

func handleNewVersion(new t.HelmChartArgs, curr t.HelmChartArgs, kubeObject *fn.KubeObject, idx int, upgradeConstraint string) {
	if new.Version != curr.Version {
		upgradesAvailable++
		anno := curr.Repo + "/" + curr.Name + ":" + new.Version
		if Config.AnnotateOnUpgradeAvailable {
			if idx >= 0 {
				kubeObject.SetAnnotation(annotationUpgradeAvailable+"."+strconv.FormatInt(int64(idx), 10), anno)
			} else {
				kubeObject.SetAnnotation(annotationUpgradeAvailable, anno)
			}
		}
		if Config.UpgradeOnUpgradeAvailable {
			upgradesDone++
			err := kubeObject.SetNestedField(new.Version, "spec", "source", "targetRevision")
			if err != nil {
				panic(err)
			}
		}
		if Config.AnnotateSumOnUpgradeAvailable {
			_, chartSum, _ := helm.PullChart(curr)
			kubeObject.SetAnnotation(annotationUpgradeShaSum, "sha256:"+chartSum)
		}
		curr_json, _ := json.Marshal(curr)
		new_json, _ := json.Marshal(new)
		fmt.Fprintf(os.Stderr, "{\"current\": %s, \"upgraded\": %s, \"constraint\": %q}\n", string(curr_json), string(new_json), upgradeConstraint)
	} else {
		if Config.AnnotateCurrentSum && kubeObject.GetAnnotation(annotationShaSum) == "" {
			_, chartSum, _ := helm.PullChart(curr)
			kubeObject.SetAnnotation(annotationShaSum, "sha256:"+chartSum)
		}
	}
}

func Run(rl *fn.ResourceList) (bool, error) {
	cfg := rl.FunctionConfig
	parseConfig(cfg)

	for _, kubeObject := range rl.Items {
		if kubeObject.IsGVK("fn.kpt.dev", "", "RenderHelmChart") {
			upgradeConstraint := kubeObject.GetAnnotation(annotationUpgradeConstraint)

			y := kubeObject.String()
			spec, err := t.ParsePktSpec([]byte(y))
			if err != nil {
				return false, err
			}
			for idx, helmChart := range spec.Charts {
				new_version, err := evaluateChartVersion(helmChart.Args, upgradeConstraint)
				if err != nil {
					return false, err
				}
				handleNewVersion(*new_version, helmChart.Args, kubeObject, idx, upgradeConstraint)
			}

		} else if kubeObject.IsGVK("argoproj.io", "", "Application") {
			upgradeConstraint := kubeObject.GetAnnotation(annotationUpgradeConstraint)

			var err error
			y := kubeObject.String()
			app, err := t.ParseArgoCDSpec([]byte(y))
			if err != nil {
				return false, err
			}
			chartArgs := app.Spec.Source.ToKptSpec()
			new_version, err := evaluateChartVersion(chartArgs, upgradeConstraint)
			if err != nil {
				return false, err
			}
			handleNewVersion(*new_version, chartArgs, kubeObject, -1, upgradeConstraint)
		}
	}

	fmt.Fprintf(os.Stderr, "{\"upgradesDone\": %d, \"upgradesAvailable\": %d, \"upgradesSkipped\": %d}\n", upgradesDone, upgradesAvailable, upgradesAvailable-upgradesDone)
	return true, nil
}

func main() {
	fmt.Fprintf(os.Stderr, "version: %s\n", version.Version)
	if err := fn.AsMain(fn.ResourceListProcessorFunc(Run)); err != nil {
		os.Exit(1)
	}
}
