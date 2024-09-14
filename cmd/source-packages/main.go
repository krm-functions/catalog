// Copyright 2024 Michael Vittrup Larsen
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
	"path/filepath"

	"github.com/krm-functions/catalog/pkg/version"

	"sigs.k8s.io/kustomize/kyaml/fn/framework"
	"sigs.k8s.io/kustomize/kyaml/fn/framework/command"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
)

type Data struct {
	Foo string `yaml:"foo,omitempty" json:"foo,omitempty"`
}

type FunctionConfig struct {
	Data Data `yaml:"data,omitempty" json:"data,omitempty"`
}

type FilterState struct {
	fnConfig *FunctionConfig
	Results  framework.Results
}

func (fnCfg *FunctionConfig) Default() error { //nolint:unparam // this return is part of the Defaulter interface
	if fnCfg.Data.Foo == "" {
		fnCfg.Data.Foo = "main"
	}
	return nil
}

func (fnCfg *FunctionConfig) Validate() error {
	return nil
}

func Processor() framework.ResourceListProcessor {
	return framework.ResourceListProcessorFunc(func(rl *framework.ResourceList) error {
		config := &FunctionConfig{}
		if err := framework.LoadFunctionConfig(rl.FunctionConfig, config); err != nil {
			return fmt.Errorf("reading function-config: %w", err)
		}
		// filter := FilterState{
		// 	fnConfig: config,
		// }

		for _, object := range rl.Items {
			if object.GetApiVersion() == "foo.bar" {
				objPath := filepath.Join(filepath.Dir(object.GetAnnotations()[kioutil.PathAnnotation]), object.GetName())
				packages, err := ParsePkgSpec(object, objPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "FIXME %v\n", err)
				}
				sources, err := packages.FetchSources("/tmp/source-packages")
				if err != nil {
					fmt.Fprintf(os.Stderr, "FIXME %v\n", err)
				}
				fmt.Fprintf(os.Stderr, "Found %v source(s)\n", len(sources))
			}
		}
			//rl.Results = append(rl.Results, filter.Results...)
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
