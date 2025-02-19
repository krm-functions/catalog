// Copyright 2025 Michael Vittrup Larsen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"fmt"
	"os"

	"github.com/krm-functions/catalog/pkg/version"

	"sigs.k8s.io/kustomize/kyaml/fn/framework"
	"sigs.k8s.io/kustomize/kyaml/fn/framework/command"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	localConfigAnno = "config.kubernetes.io/local-config"
)

func Processor() framework.ResourceListProcessor {
	return framework.ResourceListProcessorFunc(func(rl *framework.ResourceList) error {
		var res []*yaml.RNode
		for _, item := range rl.Items {
			annos := item.GetAnnotations()
			aval, ok := annos[localConfigAnno]
			if ok && aval == "true" {
				rl.Results = append(rl.Results, &framework.Result{
					Message:  fmt.Sprintf("removed %v/%v\n", item.GetKind(), item.GetName()),
					Severity: framework.Info,
				})
			} else {
				res = append(res, item)
			}
		}
		rl.Items = res
		return nil
	})
}

func main() {
	cmd := command.Build(Processor(), command.StandaloneEnabled, false)

	cmd.Version = version.Version

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
