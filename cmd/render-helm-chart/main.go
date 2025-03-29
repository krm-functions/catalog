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
	"encoding/base64"
	"fmt"
	"os"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/krm-functions/catalog/pkg/api"
	"github.com/krm-functions/catalog/pkg/helm"
	t "github.com/krm-functions/catalog/pkg/helmspecs"
	"github.com/krm-functions/catalog/pkg/util"
)

func Run(rl *fn.ResourceList) (bool, error) {
	var outputs fn.KubeObjects
	var results fn.Results

	results = append(results, &fn.Result{
		Message:  "render-helm-chart",
		Severity: fn.Info,
	})

	for _, kubeObject := range rl.Items {
		switch {
		case kubeObject.IsGVK(api.HelmResourceAPI, "", "RenderHelmChart"):
			y := kubeObject.String()
			spec, err := t.ParseKptSpec([]byte(y))
			if err != nil {
				return false, err
			}
			for idx := range spec.Charts {
				if spec.Charts[idx].Options.ReleaseName == "" {
					return false, fmt.Errorf("invalid chart spec %s: ReleaseName required, index %d", kubeObject.GetName(), idx)
				}
			}
			for idx := range spec.Charts {
				chartTarball, err := base64.StdEncoding.DecodeString(spec.Charts[idx].Chart)
				if err != nil {
					return false, err
				}
				if len(chartTarball) == 0 {
					return false, fmt.Errorf("no embedded chart found")
				}
				rendered, err := helm.Template(&spec.Charts[idx], chartTarball)
				if err != nil {
					return false, err
				}
				newobjs, err := helm.ParseAsKubeObjects(rendered)
				if err != nil {
					return false, err
				}
				outputs = append(outputs, newobjs...)
			}
		// Sourcing based on `fn.kpt.dev` is deprecated. Use the `source-helm-chart` function instead
		case kubeObject.IsGVK("fn.kpt.dev", "", "RenderHelmChart"):
			results = append(results, &fn.Result{
				Message:  "sourcing with render-helm-chart is deprecated. Use source-helm-chart instead",
				Severity: fn.Warning,
				ResourceRef: &fn.ResourceRef{
					APIVersion: kubeObject.GetAPIVersion(),
					Kind:       kubeObject.GetKind(),
					Name:       kubeObject.GetName(),
				},
				File: &fn.File{
					Path:  kubeObject.PathAnnotation(),
					Index: kubeObject.IndexAnnotation(),
				},
			})

			y := kubeObject.String()
			spec, err := t.ParseKptSpec([]byte(y))
			if err != nil {
				return false, err
			}
			for idx := range spec.Charts {
				chart := &spec.Charts[idx]
				var uname, pword string
				if chart.Args.Auth != nil {
					uname, pword, err = util.LookupAuthSecret(chart.Args.Auth.Name, chart.Args.Auth.Namespace, rl)
					if err != nil {
						return false, err
					}
				}
				chartData, _, chartSum, err := helm.SourceChart(&chart.Args, "", uname, pword)
				if err != nil {
					return false, err
				}
				err = kubeObject.SetAPIVersion(api.HelmResourceAPIVersion)
				if err != nil {
					return false, err
				}
				chs, found, err := kubeObject.NestedSlice("helmCharts")
				if !found {
					return false, fmt.Errorf("helmCharts key not found in %s", kubeObject.GetName())
				}
				if err != nil {
					return false, err
				}
				err = chs[idx].SetNestedField(base64.StdEncoding.EncodeToString(chartData), "chart")
				if err != nil {
					return false, err
				}
				err = kubeObject.SetAnnotation(api.HelmResourceAnnotationShaSum, "sha256:"+chartSum)
				if err != nil {
					return false, err
				}
			}
			outputs = append(outputs, kubeObject)
		default:
			outputs = append(outputs, kubeObject)
		}
	}

	rl.Results = results
	rl.Items = outputs
	return true, nil
}

func main() {
	if err := fn.AsMain(fn.ResourceListProcessorFunc(Run)); err != nil {
		os.Exit(1)
	}
}
