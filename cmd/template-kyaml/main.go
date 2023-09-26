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
	"errors"
	"fmt"
	"os"

	"github.com/michaelvl/krm-functions/pkg/version"

	"sigs.k8s.io/kustomize/kyaml/fn/framework"
	"sigs.k8s.io/kustomize/kyaml/fn/framework/command"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type FunctionConfig struct {
	SomeConfig string `yaml:"someConfig,omitempty" json:"someConfig,omitempty"`
}

type FilterState struct {
	Results   framework.Results
}

func (v *FunctionConfig) Default() error {
	v.SomeConfig = "SomeString"
	return nil
}

func (v *FunctionConfig) Validate() error {
	if len(v.SomeConfig)==0 {
		return fmt.Errorf("String length is zero")
	}
	return nil
}

func (f *FilterState) Each(items []*yaml.RNode) ([]*yaml.RNode, error) {
	var err error
	for _, item := range items {
		err = errors.Join(err, item.PipeE(f))
	}
	return items, err
}

func (f *FilterState) Filter(object *yaml.RNode) (*yaml.RNode, error) {
	f.Results = append(f.Results, &framework.Result{Message: fmt.Sprintf("%s/%s", object.GetKind(), object.GetName())})
	return object, nil
}

func Processor() framework.ResourceListProcessor {
	return framework.ResourceListProcessorFunc(func(rl *framework.ResourceList) error {
		config := &FunctionConfig{}
		if err := framework.LoadFunctionConfig(rl.FunctionConfig, config); err != nil {
			return fmt.Errorf("read function config: %w", err)
		}

		filter := FilterState{}

		_, err := filter.Each(rl.Items)
		rl.Results = append(rl.Results, filter.Results...)

		return err
	})
}

func main() {
	cmd := command.Build(Processor(), command.StandaloneEnabled, false)

	cmd.Version = version.Version
	//cmd.Short = generated.cmdhort
	//cmd.Long = generated.cmdLong
	//cmd.Example = generated.cmdExamples

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
