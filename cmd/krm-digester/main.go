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
	"github.com/michaelvl/krm-functions/pkg/api"
	"github.com/michaelvl/krm-functions/pkg/helm"
	t "github.com/michaelvl/krm-functions/pkg/helmspecs"
)

func Run(rl *fn.ResourceList) (bool, error) {
	var outputs fn.KubeObjects
	var results fn.Results

	results = append(results, &fn.Result{
		Message:  "digester",
		Severity: fn.Info,
	})

	for _, kubeObject := range rl.Items {
		if kubeObject.IsGVK(api.HelmResourceAPI, "", "RenderHelmChart") {
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
				objs, err := helm.ParseAsRNodes(rendered)
				if err != nil {
					return false, err
				}
				imageFilter := NewImageFilter()
				_, err = imageFilter.Filter(objs)
				if err != nil {
					return false, err
				}
				imageFilter.LookupDigests()
				for _, image := range imageFilter.Images {
					results = append(results, &fn.Result{
						Message:  fmt.Sprintf("image: %v\n", image+"@"+imageFilter.Digests[image]),
						Severity: fn.Info,
					})
				}
			}
		}
		outputs = append(outputs, kubeObject)
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
