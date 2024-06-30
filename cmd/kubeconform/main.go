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

// Template does not implement any functionality - it is merely a
// template for a KRM filter function using the kustomize yaml
// framework
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/krm-functions/catalog/pkg/version"
	"github.com/yannh/kubeconform/pkg/validator"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/kyaml/fn/framework"
	"sigs.k8s.io/kustomize/kyaml/fn/framework/command"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type FunctionConfig struct {
	SomeConfig string `yaml:"someConfig,omitempty" json:"someConfig,omitempty"`
}

type FilterState struct {
	fnConfig  *FunctionConfig
	validator validator.Validator
	Results   framework.Results
}

// LoadFunctionConfig parse the provided input, which can be a
// ConfigMap or other custom types
func (fnCfg *FunctionConfig) LoadFunctionConfig(o *yaml.RNode) error {
	if o.GetKind() == "ConfigMap" && o.GetApiVersion() == "v1" {
		var cm corev1.ConfigMap
		if err := yaml.Unmarshal([]byte(o.MustString()), &cm); err != nil {
			return err
		}
		// More mappings here ...
		fnCfg.SomeConfig = cm.Data["someConfig"]
		return nil
	}

	// Other function-config types here ...

	return fmt.Errorf("unknown function config")
}

func (f *FilterState) Each(items []*yaml.RNode) ([]*yaml.RNode, error) {
	var err error
	for _, item := range items {
		err = errors.Join(err, item.PipeE(f))
	}
	return items, err
}

// The main functionality goes here...
func (f *FilterState) Filter(object *yaml.RNode) (*yaml.RNode, error) {
	f.Results = append(f.Results, &framework.Result{Message: fmt.Sprintf("%s/%s", object.GetKind(), object.GetName())})
	fmt.Fprintf(os.Stderr, "xxx\n%s\n", object.MustString())
	return object, nil
}

func Processor() framework.ResourceListProcessor {
	return framework.ResourceListProcessorFunc(func(rl *framework.ResourceList) error {
		config := &FunctionConfig{}
		if err := config.LoadFunctionConfig(rl.FunctionConfig); err != nil {
			return fmt.Errorf("reading function-config: %w", err)
		}
		fmt.Fprintf(os.Stderr, "function-config: %+v\n", config)

		v, err := validator.New(nil, validator.Opts{Strict: true})
		if err != nil {
			return fmt.Errorf("initializing validator: %s", err)
		}
		filter := FilterState{
			fnConfig: config,
			validator: v,
		}

		_, err = filter.Each(rl.Items)
		rl.Results = append(rl.Results, filter.Results...)

		return err
	})
}

func main() {
	cmd := command.Build(Processor(), command.StandaloneEnabled, false)

	cmd.Version = version.Version

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
