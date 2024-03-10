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
	"github.com/michaelvl/krm-functions/pkg/helm"
	t "github.com/michaelvl/krm-functions/pkg/helmspecs"
)

const (
	annotationURL    = "experimental.helm.sh/"
	annotationShaSum = annotationURL + "chart-sum"
)

func Run(rl *fn.ResourceList) (bool, error) {
	var outputs fn.KubeObjects

	for _, kubeObject := range rl.Items {
		switch {
		case kubeObject.IsGVK("experimental.helm.sh", "", "RenderHelmChart"):
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
				newobjs, err := helm.Template(&spec.Charts[idx], chartTarball)
				if err != nil {
					return false, err
				}
				outputs = append(outputs, newobjs...)
			}
		case kubeObject.IsGVK("fn.kpt.dev", "", "RenderHelmChart"):
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
				err = kubeObject.SetAPIVersion("experimental.helm.sh/v1alpha1")
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
				err = chs[0].SetNestedField(base64.StdEncoding.EncodeToString(chartData), "chart")
				if err != nil {
					return false, err
				}
				err = kubeObject.SetAnnotation(annotationShaSum, "sha256:"+chartSum)
				if err != nil {
					return false, err
				}
				outputs = append(outputs, kubeObject)
			}
		default:
			outputs = append(outputs, kubeObject)
		}
	}

	rl.Items = outputs
	return true, nil
}

func main() {
	if err := fn.AsMain(fn.ResourceListProcessorFunc(Run)); err != nil {
		os.Exit(1)
	}
}
