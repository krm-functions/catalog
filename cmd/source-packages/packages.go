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
	"time"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/krm-functions/catalog/pkg/api"
	"github.com/krm-functions/catalog/pkg/git"
	"github.com/krm-functions/catalog/pkg/kpt"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

type Fleet struct {
	Spec FleetSpec `yaml:"spec,omitempty" json:"spec,omitempty"`
}

type FleetSpec struct {
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
	Upstream UpstreamID   `yaml:"upstream,omitempty" json:"upstream,omitempty"`
	Ref      SourceRef    `yaml:"ref,omitempty" json:"ref,omitempty"`
	Enabled  *bool        `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Name     string       `yaml:"name,omitempty" json:"name,omitempty"`
	SrcPath  string       `yaml:"sourcePath,omitempty" json:"sourcePath,omitempty"`
	Stub     *bool        `yaml:"stub,omitempty" json:"stub,omitempty"`
	Metadata Metadata     `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	Packages PackageSlice `yaml:"packages,omitempty" json:"packages,omitempty"`
	// Relative path of where to store package. Generally identical to 'Name'
	dstRelPath string
}

type Metadata struct {
	Mode string `yaml:"mode,omitempty" json:"mode,omitempty"`
}

type PackageSource struct {
	Type     string
	CurrRef  SourceRef
	Upstream *UpstreamGit
	Git      *git.Repository
	Path     string // Local absolute path to repo files
}

func (packages PackageSlice) Validate() error {
	for idx := range packages {
		p := &packages[idx]
		if p.Name == "" {
			return fmt.Errorf("packages must have 'name' (index %v)", idx)
		}
		if p.SrcPath == "" && !*p.Stub {
			return fmt.Errorf("Package %q needs 'path'", p.Name)
		}
		if p.SrcPath != "" && *p.Stub {
			return fmt.Errorf("Package %q cannot be a stub and have 'path'", p.Name)
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

func (packages PackageSlice) Default(defaults *PackageDefaults) {
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
		if p.Stub == nil {
			p.Stub = PtrTo(false)
		}
		if p.SrcPath == "" && p.Name != "" && !*p.Stub {
			p.SrcPath = p.Name
		}
		if p.Metadata.Mode == "" {
			p.Metadata.Mode = "kptForDeployment"
		}
		p.dstRelPath = p.Name
		p.Packages.Default(defaults)
	}
}

func (packages PackageSlice) Print(w io.Writer) {
	for idx := range packages {
		p := &packages[idx]
		fmt.Fprintf(w, "%v: %v -> %v\n", p.Name, p.SrcPath, p.dstRelPath)
		p.Packages.Print(w)
	}
}

func ParseFleetSpec(object []byte) (*Fleet, error) {
	packages := &Fleet{}
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

	packages.Spec.Packages.Default(&packages.Spec.Defaults)
	return packages, packages.Spec.Packages.Validate()
}

func NewPackageSource(u *Upstream, fileBase string) (*PackageSource, fn.Results, error) {
	var fnResults fn.Results
	localPath := filepath.Join(fileBase, string(u.Name)) // FIXME: Use better name reflecting repo name
	if u.Type == api.PackageUpstreamTypeGit {
		start := time.Now()
		r, err := git.Clone(u.Git.Repo, u.Git.AuthMethod, localPath)
		if err != nil {
			return nil, fnResults, err
		}
		t := time.Now()
		elapsed := t.Sub(start).Truncate(time.Millisecond)
		fnResults = append(fnResults, fn.GeneralResult(fmt.Sprintf("cloned %v in %v\n", u.Git.Repo, elapsed), fn.Info))
		return &PackageSource{
			Type:     api.PackageUpstreamTypeGit,
			Upstream: &u.Git,
			Git:      r,
			Path:     localPath}, fnResults, nil
	}
	return nil, fnResults, nil
}

func PackageSourceLookup(sources []PackageSource, upstream *Upstream) *PackageSource {
	for idx := range sources {
		src := &sources[idx]
		if upstream.Type == api.PackageUpstreamTypeGit {
			if upstream.Git == *src.Upstream {
				return src
			}
		}
	}
	return nil
}

// UpstreamLookup locates an upstream definition by name in a given Fleet
func UpstreamLookup(fleet *Fleet, upstream UpstreamID) *Upstream {
	for idx := range fleet.Spec.Upstreams {
		u := &fleet.Spec.Upstreams[idx]
		if u.Name == upstream {
			return u
		}
	}
	return nil
}

func SourceEnsureVersion(src *PackageSource, ref SourceRef) (fn.Results, error) {
	var fnResults fn.Results
	if src.CurrRef == ref {
		return fnResults, nil
	}
	if src.Type == api.PackageUpstreamTypeGit {
		start := time.Now()
		err := src.Git.Checkout(string(ref))
		if err != nil {
			fnResults = append(fnResults, fn.GeneralResult(fmt.Sprintf("error fetching %v @ %v\n", src.Upstream.Repo, ref), fn.Error))
			return fnResults, err
		}
		t := time.Now()
		elapsed := t.Sub(start).Truncate(time.Millisecond)
		fnResults = append(fnResults, fn.GeneralResult(fmt.Sprintf("fetched %v @ %v in %v\n", src.Upstream.Repo, ref, elapsed), fn.Info))
		src.CurrRef = ref
		return fnResults, nil
	}
	return fnResults, nil
}

// TossFiles copies package files
func (fleet *Fleet) TossFiles(sources []PackageSource, packages PackageSlice, dstAbsBasePath string) (fn.Results, error) {
	var fnResults fn.Results
	for idx := range packages {
		p := &packages[idx]
		if !*p.Enabled {
			fnResults = append(fnResults, fn.GeneralResult(fmt.Sprintf("package %v; disabled\n", p.Name), fn.Info))
			continue
		}
		d := filepath.Join(dstAbsBasePath, p.dstRelPath)
		if *p.Stub {
			fnResults = append(fnResults, fn.GeneralResult(fmt.Sprintf("package %v; stub package at %v\n", p.Name, p.dstRelPath), fn.Info))
		} else {
			u := UpstreamLookup(fleet, p.Upstream)
			if u == nil {
				return fnResults, fmt.Errorf("unknown upstream: %v", p.Upstream)
			}
			src := PackageSourceLookup(sources, u)
			if src == nil { // FIXME: This should not happen
				return fnResults, fmt.Errorf("unknown upstream source: %v", p.Upstream)
			}
			fnRes, err := SourceEnsureVersion(src, p.Ref)
			fnResults = append(fnResults, fnRes...)
			if err != nil {
				return fnResults, err
			}
			fnResults = append(fnResults, fn.GeneralResult(fmt.Sprintf("package %v; %v --> %v\n", p.Name, p.SrcPath, p.dstRelPath), fn.Info))
			s := filepath.Join(src.Path, p.SrcPath)
			err = os.CopyFS(d, os.DirFS(s))
			if err != nil {
				return fnResults, fmt.Errorf("copying package dir (%v --> %v): %v", src.Path, d, err)
			}
			if p.Metadata.Mode == "kptForDeployment" {
				// FIXME assumes git upstream
				err := kpt.UpdateKptMetadata(d, p.Name, p.SrcPath, u.Git.Repo, string(p.Ref))
				if err != nil {
					return fnResults, fmt.Errorf("mutating package %v metadata: %v", p.Name, err)
				}
			}
		}
		fnRes, err := fleet.TossFiles(sources, p.Packages, d)
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
		PreserveSeqIndent: false,
		WrapBareSeqNode:   true,
	}
	return pr.Read()
}

func PtrTo[T any](val T) *T {
	return &val
}
