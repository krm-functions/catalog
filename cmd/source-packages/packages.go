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

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/krm-functions/catalog/pkg/api"
	"github.com/krm-functions/catalog/pkg/git"
	"github.com/krm-functions/catalog/pkg/kpt"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Packages struct {
	Spec PackagesSpec `yaml:"spec,omitempty" json:"spec,omitempty"`
}

type PackagesSpec struct {
	Upstreams []Upstream      `yaml:"upstreams,omitempty" json:"upstreams,omitempty"`
	Defaults  PackageDefaults `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	Packages  PackageSlice    `yaml:"packages,omitempty" json:"packages,omitempty"`
}

type PackageSlice []Package

type UpstreamID string

type SourceRef string

type Upstream struct {
	Name UpstreamID  `yaml:"name,omitempty" json:"name,omitempty"`
	Type string      `yaml:"type,omitempty" json:"type,omitempty"`
	Git  UpstreamGit `yaml:"git,omitempty" json:"git,omitempty"`
}

type UpstreamGit struct {
	Repo       string `yaml:"repo,omitempty" json:"repo,omitempty"`
	AuthMethod string `yaml:"authMethod,omitempty" json:"authMethod,omitempty"`
}

type PackageDefaults struct {
	Upstream UpstreamID `yaml:"upstream,omitempty" json:"upstream,omitempty"`
	Ref      SourceRef  `yaml:"ref,omitempty" json:"ref,omitempty"`
	Enabled  *bool      `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

type Package struct {
	Upstream   UpstreamID   `yaml:"upstream,omitempty" json:"upstream,omitempty"`
	Ref        SourceRef    `yaml:"ref,omitempty" json:"ref,omitempty"`
	Enabled    *bool        `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Name       string       `yaml:"name,omitempty" json:"name,omitempty"`
	SrcPath    string       `yaml:"sourcePath,omitempty" json:"sourcePath,omitempty"`
	Empty      *bool        `yaml:"empty,omitempty" json:"empty,omitempty"`
	Metadata   Metadata     `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Packages   PackageSlice `yaml:"packages,omitempty" json:"packages,omitempty"`
	dstRelPath string
}

type Metadata struct {
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`
}

type PackageSource struct {
	Upstream *Upstream
	Git      *git.Repository
}

func (packages PackageSlice) Validate() error {
	for idx := range packages {
		p := &packages[idx]
		if p.Name == "" {
			return fmt.Errorf("Packages must have 'name' (index %v)", idx)
		}
		if p.SrcPath == "" && !*p.Empty {
			return fmt.Errorf("Package %q needs 'path'", p.Name)
		}
		if p.SrcPath != "" && *p.Empty {
			return fmt.Errorf("Package %q cannot be empty and have 'path'", p.Name)
		}
		if p.Upstream == "" {
			return fmt.Errorf("Package %q has no upstream", p.Name)
		}
		if p.Ref == "" {
			return fmt.Errorf("Package %q has no ref", p.Name)
		}
		if err := p.Packages.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (packages PackageSlice) Default(defaults *PackageDefaults, basePath string) {
	for idx := range packages {
		p := &packages[idx]
		if p.Upstream == "" {
			p.Upstream = defaults.Upstream
		}
		if p.Ref == "" {
			p.Ref = defaults.Ref
		}
		if p.Enabled == nil {
			p.Enabled = PtrTo(*defaults.Enabled)
		}
		if p.Empty == nil {
			p.Empty = PtrTo(false)
		}
		if p.SrcPath == "" && p.Name != "" && !*p.Empty {
			p.SrcPath = p.Name
		}
		if p.Metadata.Mode == "" {
			p.Metadata.Mode = "kptForDeployment"
		}
		p.dstRelPath = p.Name
		p.Packages.Default(defaults, p.dstRelPath)
	}
}

func (packages PackageSlice) Print(w io.Writer) {
	for idx := range packages {
		p := &packages[idx]
		fmt.Fprintf(w, "%v: %v -> %v\n", p.Name, p.SrcPath, p.dstRelPath)
		p.Packages.Print(w)
	}
}

func ParsePkgSpec(object []byte) (*Packages, error) {
	packages := &Packages{}
	if err := yaml.Unmarshal(object, packages); err != nil {
		return nil, err
	}

	// Defaults for defaults
	if packages.Spec.Defaults.Enabled == nil {
		packages.Spec.Defaults.Enabled = PtrTo(true)
	}
	if packages.Spec.Defaults.Upstream == "" && len(packages.Spec.Upstreams) == 1 {
		packages.Spec.Defaults.Upstream = packages.Spec.Upstreams[0].Name
	}

	packages.Spec.Packages.Default(&packages.Spec.Defaults, "")
	return packages, packages.Spec.Packages.Validate()
}

func (packages *Packages) FetchSources(fileBase string) ([]PackageSource, error) {
	repos := make([]PackageSource, 0)
	for idx := range packages.Spec.Upstreams {
		u := &packages.Spec.Upstreams[idx]
		if u.Type == api.PackageUpstreamTypeGit {
			r, err := git.Clone(u.Git.Repo, u.Git.AuthMethod, fileBase)
			if err != nil {
				return nil, err
			}
			rr := PackageSource{
				Upstream: u,
				Git:      r}
			repos = append(repos, rr)
		}
	}
	return repos, nil
}

func SourceLookup(sources []PackageSource, upstream UpstreamID) *PackageSource {
	for _, src := range sources {
		if src.Upstream.Name == upstream {
			return &src
		}
	}
	return nil
}

func SourceEnsureVersion(sources []PackageSource, upstream UpstreamID, ref SourceRef) (*PackageSource, error) {
	src := SourceLookup(sources, upstream)
	if src.Upstream.Type == api.PackageUpstreamTypeGit {
		err := src.Git.Checkout(string(ref))
		if err != nil {
			return nil, err
		}
		return src, nil
	}
	return nil, nil
}

// TossFiles copies package files
func (packages PackageSlice) TossFiles(sources []PackageSource, srcBasePath, dstBasePath string) (fn.Results, error) {
	var fnResults fn.Results
	for idx := range packages {
		p := &packages[idx]
		if !*p.Enabled {
			fnResults = append(fnResults, fn.GeneralResult(fmt.Sprintf("package %v; disabled\n", p.Name), fn.Info))
			continue
		}
		d := filepath.Join(dstBasePath, p.dstRelPath)
		if *p.Empty {
			fnResults = append(fnResults, fn.GeneralResult(fmt.Sprintf("package %v; empty package at dstPath:%v\n", p.Name, d), fn.Info))
		} else {
			fnResults = append(fnResults, fn.GeneralResult(fmt.Sprintf("package %v; srcPath:%v dstPath:%v\n", p.Name, p.SrcPath, d), fn.Info))
			src, err := SourceEnsureVersion(sources, p.Upstream, p.Ref)
			if err != nil {
				return fnResults, fmt.Errorf("git checkout %v @ %v: %v", src.Git.Repo, p.Ref, err)
			}
			s := filepath.Join(srcBasePath, p.SrcPath)
			err = os.CopyFS(d, os.DirFS(s))
			if err != nil {
				return fnResults, fmt.Errorf("copying package dir (%v -> %v): %v", s, d, err)
			}
			if p.Metadata.Mode == "kptForDeployment" {
				// FIXME assumes git upstream
				err := kpt.UpdateKptMetadata(d, p.Name, p.SrcPath, src.Upstream.Git.Repo, string(p.Ref))
				if err != nil {
					return fnResults, fmt.Errorf("mutating package %v metadata: %v", p.Name, err)
				}
			}
		}
		fnRes, err := p.Packages.TossFiles(sources, srcBasePath, d)
		if err != nil {
			return fnResults, fmt.Errorf("tossing child packages for %v: %v", p.Name, err)
		}
		fnResults = append(fnResults, fnRes...)
	}
	return fnResults, nil
}

func FilesystemToObjects(path string) ([]*yaml.RNode, error) {
	pr := &kio.LocalPackageReader{
		PackagePath:       path,
		MatchFilesGlob:    []string{"*"},
		PreserveSeqIndent: true,
		WrapBareSeqNode:   true,
	}
	return pr.Read()
}

func PtrTo[T any](val T) *T {
	return &val
}
