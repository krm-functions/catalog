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
	"fmt"

	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Packages struct {
	Spec PackagesSpec `yaml:"spec,omitempty" json:"spec,omitempty"`
}

type PackagesSpec struct {
	Defaults PackageDefaultable `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	Packages []Package          `yaml:"packages,omitempty" json:"packages,omitempty"`
}

type PackageDefaultable struct {
	Uri     string `yaml:"uri,omitempty" json:"uri,omitempty"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
	Enabled *bool  `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

type Package struct {
	PackageDefaultable
	Name    string `yaml:"name,omitempty" json:"name,omitempty"`
	Path    string `yaml:"path,omitempty" json:"path,omitempty"`
}

func (packages *Packages) Validate() error {
	for idx := range packages.Spec.Packages {
		p := &packages.Spec.Packages[idx]
		if p.Name == "" {
			return fmt.Errorf("Packages must have 'name' (index %v)", idx)
		}
		if p.Path == "" {
			return fmt.Errorf("Package %q needs 'path'", p.Name)
		}
	}
	return nil
}

func ParsePkgSpec(object *yaml.RNode) (*Packages, error) {
	packages := &Packages{}
	if err := yaml.Unmarshal([]byte(object.MustString()), packages); err != nil {
		return nil, err
	}
	for idx := range packages.Spec.Packages {
		p := &packages.Spec.Packages[idx]
		if p.Uri == "" {
			p.Uri = packages.Spec.Defaults.Uri
		}
		if p.Version == "" {
			p.Version = packages.Spec.Defaults.Uri
		}
		if p.Enabled == nil {
			p.Enabled = packages.Spec.Defaults.Enabled
		}
		if p.Path == "" && p.Name != "" {
			p.Path = p.Name
		}
	}
	return packages, packages.Validate()
}
