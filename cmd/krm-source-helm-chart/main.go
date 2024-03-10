// Copyright 2024 Michael Vittrup Larsen
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
	"github.com/michaelvl/krm-functions/pkg/helm"
	t "github.com/michaelvl/krm-functions/pkg/helmspecs"
)

const (
	annotationURL    = apiGroup
	annotationShaSum = annotationURL + "/chart-sum"
	apiGroup         = "experimental.helm.sh"
	apiVersion       = apiGroup + "/v1alpha1"
)

func Run(rl *fn.ResourceList) (bool, error) {
	var outputs fn.KubeObjects

	for _, kubeObject := range rl.Items {
		if kubeObject.IsGVK(apiGroup, "", "RenderHelmChart") || kubeObject.IsGVK("fn.kpt.dev", "", "RenderHelmChart") {
			y := kubeObject.String()
			spec, err := t.ParseKptSpec([]byte(y))
			if err != nil {
				return false, err
			}
			for idx := range spec.Charts {
				chart := &spec.Charts[idx]
				var uname, pword *string
				if chart.Args.Auth != nil {
					uname, pword, err = helm.LookupAuthSecret(chart.Args.Auth.Name, chart.Args.Auth.Namespace, rl)
					if err != nil {
						return false, err
					}
				}
				chartData, chartSum, err := helm.SourceChart(chart, uname, pword)
				if err != nil {
					return false, err
				}
				err = kubeObject.SetAPIVersion(apiVersion)
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
				err = kubeObject.SetAnnotation(annotationShaSum+"/"+chart.Args.Name, "sha256:"+chartSum)
				if err != nil {
					return false, err
				}
			}
		}
		outputs = append(outputs, kubeObject)
	}

	rl.Items = outputs
	return true, nil
}

func main() {
	if err := fn.AsMain(fn.ResourceListProcessorFunc(Run)); err != nil {
		os.Exit(1)
	}
}
