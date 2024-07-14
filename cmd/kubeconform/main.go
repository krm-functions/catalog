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
	"regexp"
	"strings"

	"github.com/krm-functions/catalog/pkg/version"
	"github.com/yannh/kubeconform/pkg/resource"
	"github.com/yannh/kubeconform/pkg/validator"

	"sigs.k8s.io/kustomize/kyaml/fn/framework"
	"sigs.k8s.io/kustomize/kyaml/fn/framework/command"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Data struct {
	KubernetesVersion    string `yaml:"kubernetes_version,omitempty" json:"kubernetes_version,omitempty"`
	IgnoreMissingSchemas string `yaml:"ignore_missing_schemas,omitempty" json:"ignore_missing_schemas,omitempty"`
	Strict               string `yaml:"strict,omitempty" json:"strict,omitempty"`
	SchemaLocations      string `yaml:"schema_locations,omitempty" json:"schema_locations,omitempty"`
}

type FunctionConfig struct {
	Data Data `yaml:"data,omitempty" json:"data,omitempty"`
}

type Stats struct {
	Resources int
	Invalid   int
	Errors    int
}

type FilterState struct {
	fnConfig  *FunctionConfig
	validator validator.Validator
	Results   framework.Results
	Stats
}

const (
	StringTrue  = "true"
	StringFalse = "false"
)

func (fnCfg *FunctionConfig) Default() error { //nolint:unparam // this return is part of the Defaulter interface
	if fnCfg.Data.KubernetesVersion == "" {
		fnCfg.Data.KubernetesVersion = "master"
	}
	if fnCfg.Data.IgnoreMissingSchemas == "" {
		fnCfg.Data.IgnoreMissingSchemas = StringFalse
	}
	if fnCfg.Data.Strict == "" {
		fnCfg.Data.Strict = StringTrue
	}
	if fnCfg.Data.SchemaLocations == "" {
		fnCfg.Data.SchemaLocations = os.Getenv("KUBECONFORM_SCHEMA_LOCATIONS")
	}
	return nil
}

func (fnCfg *FunctionConfig) Validate() error {
	if fnCfg.Data.KubernetesVersion != "master" {
		match, err := regexp.MatchString("^[0-9]+\\.[0-9]+\\.[0-9]+$", fnCfg.Data.KubernetesVersion)
		if err != nil || !match {
			return fmt.Errorf("illegal 'ignore_missing_schemas' argument: %s", fnCfg.Data.KubernetesVersion)
		}
	}
	if fnCfg.Data.IgnoreMissingSchemas != StringTrue && fnCfg.Data.IgnoreMissingSchemas != StringFalse {
		return fmt.Errorf("illegal 'ignore_missing_schemas' argument: %s", fnCfg.Data.IgnoreMissingSchemas)
	}
	if fnCfg.Data.Strict != StringTrue && fnCfg.Data.Strict != StringFalse {
		return fmt.Errorf("illegal 'strict' argument: %s", fnCfg.Data.Strict)
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
	f.Stats.Resources++
	objPath := object.GetAnnotations()[kioutil.PathAnnotation]
	res := resource.Resource{
		Path:  objPath,
		Bytes: []byte(object.MustString()),
	}
	r := f.validator.ValidateResource(res)
	var err error
	switch r.Status {
	case validator.Valid, validator.Skipped:
		f.Results = append(f.Results, &framework.Result{Message: fmt.Sprintf("%s/%s", object.GetKind(), object.GetName())})
	case validator.Invalid:
		f.Stats.Invalid++
		for _, ve := range r.ValidationErrors {
			msg := fmt.Sprintf("%s: %s\n", ve.Path, ve.Msg)
			f.Results = append(f.Results, &framework.Result{
				Severity: framework.Error,
				Message:  msg,
				ResourceRef: &yaml.ResourceIdentifier{
					TypeMeta: yaml.TypeMeta{
						APIVersion: object.GetApiVersion(),
						Kind:       object.GetKind(),
					},
					NameMeta: yaml.NameMeta{
						Name:      object.GetName(),
						Namespace: object.GetNamespace(),
					},
				},
				Field: &framework.Field{Path: objPath}})
		}
		err = fmt.Errorf("invalid %s/%s", object.GetKind(), object.GetName())
	case validator.Error: // FIXME, combine with above
		f.Stats.Errors++
		msg := fmt.Sprintf("%s\n", r.Err)
		f.Results = append(f.Results, &framework.Result{
			Severity: framework.Error,
			Message:  msg,
			ResourceRef: &yaml.ResourceIdentifier{
				TypeMeta: yaml.TypeMeta{
					APIVersion: object.GetApiVersion(),
					Kind:       object.GetKind(),
				},
				NameMeta: yaml.NameMeta{
					Name:      object.GetName(),
					Namespace: object.GetNamespace(),
				},
			},
			Field: &framework.Field{Path: objPath}})
		err = fmt.Errorf("unable to validate %s/%s", object.GetKind(), object.GetName())
	case validator.Empty:
	}
	return object, err
}

func Processor() framework.ResourceListProcessor {
	return framework.ResourceListProcessorFunc(func(rl *framework.ResourceList) error {
		config := &FunctionConfig{}
		if err := framework.LoadFunctionConfig(rl.FunctionConfig, config); err != nil {
			return fmt.Errorf("reading function-config: %w", err)
		}
		fmt.Fprintf(os.Stderr, "function-config: %+v\n", config)
		opts := validator.Opts{
			KubernetesVersion:    config.Data.KubernetesVersion,
			Strict:               config.Data.Strict == "true",
			IgnoreMissingSchemas: config.Data.IgnoreMissingSchemas == "true",
		}
		var schemas []string
		if config.Data.SchemaLocations != "" {
			schemas = append(schemas, strings.Split(config.Data.SchemaLocations, ",")...)
		}
		v, err := validator.New(schemas, opts)
		if err != nil {
			return fmt.Errorf("initializing validator: %s", err)
		}
		filter := FilterState{
			fnConfig:  config,
			validator: v,
		}

		_, err = filter.Each(rl.Items)
		rl.Results = append(rl.Results, filter.Results...)
		rl.Results = append(rl.Results, &framework.Result{Message: fmt.Sprintf("Stats: %+v", filter.Stats)})

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
