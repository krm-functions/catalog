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
	"github.com/krm-functions/catalog/pkg/kpt"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
)

type Packages struct {
	Spec PackagesSpec `yaml:"spec,omitempty" json:"spec,omitempty"`
}

type PackagesSpec struct {
	Defaults PackageDefaultable `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	Packages PackageSlice       `yaml:"packages,omitempty" json:"packages,omitempty"`
}

type PackageSlice []Package

type Upstream struct {
	Type string      `yaml:"type,omitempty" json:"type,omitempty"`
	Git  UpstreamGit `yaml:"git,omitempty" json:"git,omitempty"`
}

type UpstreamGit struct {
	Repo string  `yaml:"repo,omitempty" json:"repo,omitempty"`
	Ref string  `yaml:"ref,omitempty" json:"ref,omitempty"`
}

type PackageDefaultable struct {
	Upstream Upstream `yaml:"upstream,omitempty" json:"upstream,omitempty"`
	Enabled  *bool    `yaml:"enabled,omitempty" json:"enabled,omitempty"`
}

type Package struct {
	PackageDefaultable
	Name       string       `yaml:"name,omitempty" json:"name,omitempty"`
	SrcPath    string       `yaml:"sourcePath,omitempty" json:"sourcePath,omitempty"`
	Stub       *bool        `yaml:"stub,omitempty" json:"stub,omitempty"`
	Metadata                `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Packages   PackageSlice `yaml:"packages,omitempty" json:"packages,omitempty"`
	dstRelPath string
}

type Metadata struct {
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`
}

type PackageSource struct {
	Upstream
	Git      *git.Repository
}

func (packages PackageSlice) Validate() error {
	for idx := range packages {
		p := &packages[idx]
		if p.Name == "" {
			return fmt.Errorf("Packages must have 'name' (index %v)", idx)
		}
		if p.SrcPath == ""  && !*p.Stub {
			return fmt.Errorf("Package %q needs 'path'", p.Name)
		}
		if p.SrcPath != "" && *p.Stub {
			return fmt.Errorf("Package %q cannot be stub and have 'path'", p.Name)
		}
		if p.Upstream.Type != "git" {
			return fmt.Errorf("Package %q unsupported upstream type: %v", p.Upstream.Type)
		}
		if err := p.Packages.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func (packages PackageSlice) Default(defaults *PackageDefaultable, basePath string) {
	var isStub = false
	for idx := range packages {
		p := &packages[idx]
		if p.Upstream.Type == "" {
			p.Upstream.Type = defaults.Upstream.Type
		}
		if p.Upstream.Type == "git" {
			if p.Upstream.Git.Repo == "" {
				p.Upstream.Git.Repo = defaults.Upstream.Git.Repo
			}
			if p.Upstream.Git.Ref == "" {
				p.Upstream.Git.Ref = defaults.Upstream.Git.Ref
			}
		}
		if p.Enabled == nil {
			p.Enabled = defaults.Enabled
		}
		if p.Stub == nil {
			p.Stub = &isStub
		}
		if p.SrcPath == "" && p.Name != "" && !*p.Stub {
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
	for _, p := range packages {
		fmt.Fprintf(w, "%v: %v -> %v\n", p.Name, p.SrcPath, p.dstRelPath)
		p.Packages.Print(w)
	}
}

func ParsePkgSpec(object []byte, basePath string) (*Packages, error) {
	packages := &Packages{}
	if err := yaml.Unmarshal(object, packages); err != nil {
		return nil, err
	}
	packages.Spec.Packages.Default(&packages.Spec.Defaults, "")
	return packages, packages.Spec.Packages.Validate()
}

func (packages Packages) FetchSources(fileBase string) ([]PackageSource, error) {
	repos := make([]PackageSource, 0)
	for _, p := range packages.Spec.Packages {
		if SourceLookup(repos, p.Upstream) == nil {
			if p.Upstream.Type == "git" {
				r, err := git.Clone(p.Upstream.Git.Repo, fileBase)
				if err != nil {
					return nil, err
				}
				err = r.Checkout(p.Upstream.Git.Ref)
				if err != nil {
					return nil, err
				}
				rr := PackageSource{Upstream: p.Upstream, Git: r}
				repos = append(repos, rr)
			}
		}
	}
	return repos, nil
}

func SourceLookup(sources []PackageSource, upstream Upstream) *PackageSource {
	for _, src := range sources {
		if src.Upstream.Type == "git" {
			if src.Upstream.Git.Repo == upstream.Git.Repo {
				return &src
			}
		}
	}
	return nil
}

func SourceEnsureVersion(sources []PackageSource, upstream Upstream) error {
	src := SourceLookup(sources, upstream)
	if src.Upstream.Type == "git" {
		err := src.Git.Checkout(src.Upstream.Git.Ref)
		if err != nil {
			return err
		}
	}
	return nil
}

// TossFiles copies package files
func (packages PackageSlice) TossFiles(sources []PackageSource, srcBasePath, dstBasePath string) (fn.Results, error) {
	var fnResults fn.Results
	for _, p := range packages {
		if *p.Enabled {
			d := filepath.Join(dstBasePath, p.dstRelPath)
			fnResults = append(fnResults, fn.GeneralResult(fmt.Sprintf("package %v; srcPath:%v dstPath:%v\n", p.Name, p.SrcPath, d), fn.Info))
			if !*p.Stub {
				err := SourceEnsureVersion(sources, p.Upstream)
				if err != nil {
					return fnResults, fmt.Errorf("git checkout %v @ %v: ", p.Upstream.Git.Repo, p.Upstream.Git.Ref, err)
				}
				s := filepath.Join(srcBasePath, p.SrcPath)
				//fmt.Fprintf(os.Stderr, ">> CopyFS src:%v dst:%v\n", s, d)
				err = os.CopyFS(d, os.DirFS(s))
				if err != nil {
					return fnResults, fmt.Errorf("copying package dir: %v", err)
				}
			}
			if p.Metadata.Mode == "kptForDeployment" {
				// FIXME assumes git upstream
				kpt.UpdateKptMetadata(d, p.Name, p.SrcPath, p.Upstream.Git.Repo, p.Upstream.Git.Ref)
			}
			p.Packages.TossFiles(sources, srcBasePath, d)
		}
	}
	return fnResults, nil
}

func FilesystemToObjects(path string) ([]*yaml.RNode, error) {
	pr := &kio.LocalPackageReader{
		PackagePath: path,
		MatchFilesGlob: []string{"*"},
		PreserveSeqIndent: true,
		WrapBareSeqNode: true,
	}
	return pr.Read()
}
