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

const (
	enforcementActionKey = "enforcementAction"
	constraintApiVersion = "constraints.gatekeeper.sh/v1beta1"
)

type FilterState struct {
	enforcementAction string
	Results           framework.Results
}

func (fnCfg *FilterState) LoadFunctionConfig(o *yaml.RNode) error {
	if o.GetKind() == "ConfigMap" && o.GetApiVersion() == "v1" {
		var cm corev1.ConfigMap
		if err := yaml.Unmarshal([]byte(o.MustString()), &cm); err != nil {
			return err
		}
		fnCfg.enforcementAction = cm.Data[enforcementActionKey]
		if fnCfg.enforcementAction != "deny" &&
			fnCfg.enforcementAction != "warn" &&
			fnCfg.enforcementAction != "dryrun" {
			return fmt.Errorf("unknown enforcementAction: %v", fnCfg.enforcementAction)
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
	meta, err := object.GetMeta()
	if err != nil {
		return nil, fmt.Errorf("could not read object metadata: %v", err)
	}
	if meta.APIVersion != constraintApiVersion {
		return object, nil
	}
	err = object.SetMapField(yaml.NewScalarRNode(f.enforcementAction), "spec", "enforcementAction")
	if err != nil {
		return nil, fmt.Errorf("cannot set enforcementAction field: %v", err)
	}
	return object, nil
}

func Processor() framework.ResourceListProcessor {
	return framework.ResourceListProcessorFunc(func(rl *framework.ResourceList) error {
		filter := &FilterState{}
		if err := filter.LoadFunctionConfig(rl.FunctionConfig); err != nil {
			return fmt.Errorf("reading function-config: %w", err)
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
