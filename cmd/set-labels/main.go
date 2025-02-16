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
	"errors"
	"fmt"
	"os"

	"github.com/krm-functions/catalog/pkg/version"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/kustomize/kyaml/fn/framework"
	"sigs.k8s.io/kustomize/kyaml/fn/framework/command"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type SetLabels struct {
	Labels            map[string]string `json:"labels,omitempty"`
	SetSelectorLabels *bool             `json:"setSelectorLabels,omitempty"`
}

type FilterState struct {
	fnConfig *SetLabels
	Results  framework.Results
}

func (fnCfg *SetLabels) LoadFunctionConfig(o *yaml.RNode) error {
	if o.GetKind() == "ConfigMap" && o.GetApiVersion() == "v1" {
		var cm corev1.ConfigMap
		if err := yaml.Unmarshal([]byte(o.MustString()), &cm); err != nil {
			return err
		}
		fnCfg.Labels = cm.Data
		return nil
	} else if o.GetKind() == "SetLabels" && o.GetApiVersion() == "fn.kpt.dev/v1alpha1" {
		if err := yaml.Unmarshal([]byte(o.MustString()), &fnCfg); err != nil {
			return err
		}
		if fnCfg.SetSelectorLabels != nil && *fnCfg.SetSelectorLabels {
			return fmt.Errorf("function does not support setting selector labels")
		}
		return nil
	}
	return fmt.Errorf("unknown function config")
}

func (f *FilterState) Each(items []*yaml.RNode) ([]*yaml.RNode, error) {
	var err error
	for _, item := range items {
		err = errors.Join(err, item.PipeE(f))
	}
	return items, err
}

func (f *FilterState) Filter(object *yaml.RNode) (*yaml.RNode, error) {
	err := object.SetLabels(f.fnConfig.Labels)
	if err != nil {
		return object, err
	}
	return object, nil
}

func Processor() framework.ResourceListProcessor {
	return framework.ResourceListProcessorFunc(func(rl *framework.ResourceList) error {
		config := &SetLabels{}
		if err := config.LoadFunctionConfig(rl.FunctionConfig); err != nil {
			return fmt.Errorf("reading function-config: %w", err)
		}

		filter := FilterState{
			fnConfig: config,
		}

		_, err := filter.Each(rl.Items)
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
