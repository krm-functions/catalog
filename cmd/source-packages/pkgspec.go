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
	"io"
	"os"
	"path/filepath"

	"github.com/krm-functions/catalog/pkg/git"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Packages struct {
	Spec PackagesSpec `yaml:"spec,omitempty" json:"spec,omitempty"`
}

type PackagesSpec struct {
	Defaults PackageDefaultable `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	Packages PackageSlice       `yaml:"packages,omitempty" json:"packages,omitempty"`
}

type PackageSlice []Package

type PackageDefaultable struct {
	URI     string `yaml:"uri,omitempty" json:"uri,omitempty"`
	Version string `yaml:"version,omitempty" json:"version,omitempty"`
	Enabled *bool  `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

type Package struct {
	PackageDefaultable
	Name     string       `yaml:"name,omitempty" json:"name,omitempty"`
	Path     string       `yaml:"path,omitempty" json:"path,omitempty"`
	Packages PackageSlice `yaml:"packages,omitempty" json:"packages,omitempty"`
	dstPath  string
}

type Revision string

type PackageSource interface {
	GetURI() string
	SetRevision(Revision) error
}

type GitPackageSource struct {
	git.Repository
}

func (packages PackageSlice) Validate() error {
	for idx := range packages {
		p := &packages[idx]
		if p.Name == "" {
			return fmt.Errorf("Packages must have 'name' (index %v)", idx)
		}
		if p.Path == "" {
			return fmt.Errorf("Package %q needs 'path'", p.Name)
		}
		if err := p.Packages.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (packages PackageSlice) Default(defaults *PackageDefaultable, basePath string) {
	for idx := range packages {
		p := &packages[idx]
		if p.URI == "" {
			p.URI = defaults.URI
		}
		if p.Version == "" {
			p.Version = defaults.Version
		}
		if p.Enabled == nil {
			p.Enabled = defaults.Enabled
		}
		if p.Path == "" && p.Name != "" {
			p.Path = p.Name
		}
		p.dstPath = filepath.Join(basePath, p.Name)
		p.Packages.Default(defaults, p.dstPath)
	}
}

func (packages PackageSlice) Print(w io.Writer) {
	for _, p := range packages {
		fmt.Fprintf(w, "%v: %v -> %v\n", p.Name, p.Path, p.dstPath)
		p.Packages.Print(w)
	}
}

func ParsePkgSpec(object *yaml.RNode, basePath string) (*Packages, error) {
	packages := &Packages{}
	if err := yaml.Unmarshal([]byte(object.MustString()), packages); err != nil {
		return nil, err
	}
	packages.Spec.Packages.Default(&packages.Spec.Defaults, basePath)
	return packages, packages.Spec.Packages.Validate()
}

func (packages Packages) FetchSources(fileBase string) ([]PackageSource, error) {
	repos := make([]PackageSource, 0)
	for _, p := range packages.Spec.Packages {
		fmt.Fprintf(os.Stderr, "%v: %v -> %v\n", p.Name, p.Path, p.dstPath)

		if LookupSource(repos, p.URI) == nil {
			r, err := git.Clone(p.URI, fileBase)
			if err != nil {
				return nil, err
			}
			err = r.Checkout(p.Version)
			if err != nil {
				return nil, err
			}
			rr := GitPackageSource{*r}
			repos = append(repos, rr)
		}
	}
	return repos, nil
}

func LookupSource(sources []PackageSource, uri string) PackageSource {
	for _, src := range sources {
		if src.GetURI() == uri {
			return src
		}
	}
	return nil
}

func (repo GitPackageSource) GetURI() string {
	return repo.URI
}

func (repo GitPackageSource) SetRevision(rev Revision) error {
	return repo.Checkout(string(rev))
}
