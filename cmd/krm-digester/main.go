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
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

func digester(items fn.KubeObjects) error {
	imageFilter := &ImageFilter{}
	var err error
	for idx, obj := range items {
		objRN, err := yaml.Parse(obj.String())
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "** Obj %v %v/%v\n", idx, objRN.GetKind(), objRN.GetName())
		if objRN.GetKind() == "CronJob" {
			_, err = objRN.Pipe(
				yaml.Lookup("spec", "jobTemplate", "spec", "template", "spec"),
				yaml.Tee(yaml.Lookup("containers"), imageFilter),
				yaml.Tee(yaml.Lookup("initContainers"), imageFilter),
			)
		}
		_, err = objRN.Pipe(
			yaml.Lookup("spec"),
			yaml.Tee(yaml.Lookup("containers"), imageFilter),
			yaml.Tee(yaml.Lookup("initContainers"), imageFilter),
			yaml.Lookup("template", "spec"),
			yaml.Tee(yaml.Lookup("containers"), imageFilter),
			yaml.Tee(yaml.Lookup("initContainers"), imageFilter),
		)
	}
	fmt.Fprintf(os.Stderr, "** images %v\n", imageFilter.Images)
	return err
}

type ImageFilter struct {
	// List of images found traversing resources
	Images []string
}

func (f *ImageFilter) Filter(n *yaml.RNode) (*yaml.RNode, error) {
	if err := n.VisitElements(f.filterImage); err != nil {
		return nil, err
	}
	return n, nil
}

func (f *ImageFilter) filterImage(n *yaml.RNode) error {
	imageNode, err := n.Pipe(yaml.Lookup("image"))
	if err != nil {
		s, _ := n.String()
		return fmt.Errorf("could not lookup image in node %v: %w", s, err)
	}
	image := yaml.GetValue(imageNode)
	f.Images = append(f.Images, image)
	return nil
}

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
				newobjs, err := helm.Template(&spec.Charts[idx], chartTarball)
				if err != nil {
					return false, err
				}
				err = digester(newobjs)
				if err != nil {
					return false, err
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
