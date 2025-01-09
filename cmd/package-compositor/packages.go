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
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"text/template"
	"time"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/Masterminds/sprig/v3"
	"github.com/krm-functions/catalog/pkg/api"
	"github.com/krm-functions/catalog/pkg/git"
	"github.com/krm-functions/catalog/pkg/kpt"
	"github.com/krm-functions/catalog/pkg/util"
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

type Auth struct {
	Kind      string `yaml:"kind,omitempty" json:"kind,omitempty"`
	Name      string `yaml:"name,omitempty" json:"name,omitempty"`
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
}

type UpstreamGit struct {
	Repo       string `yaml:"repo,omitempty" json:"repo,omitempty"`
	AuthMethod string `yaml:"authMethod,omitempty" json:"authMethod,omitempty"`
	Auth       *Auth  `yaml:"auth,omitempty" json:"auth,omitempty"`
}

type PackageDefaults struct {
	Upstream UpstreamID `yaml:"upstream,omitempty" json:"upstream,omitempty"`
	Ref      SourceRef  `yaml:"ref,omitempty" json:"ref,omitempty"`
	Enabled  *bool      `yaml:"enabled,omitempty" json:"enabled,omitempty"`
	Metadata Metadata   `yaml:"metadata,omitempty" json:"metadata,omitempty"`
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
	// hierarchical path, i.e. including parent package paths
	dstAbsPath string
}

type Metadata struct {
	Mode              string            `yaml:"mode,omitempty" json:"mode,omitempty"`
	Spec              map[string]string `yaml:"spec,omitempty" json:"spec,omitempty"`
	Templated         map[string]string `yaml:"templated,omitempty" json:"templated,omitempty"`
	InheritFromParent *bool             `yaml:"inheritFromParent" json:"inheritFromParent"`
	mergedSpec        map[string]string // Spec, merged with parent spec
	mergedTemplated   map[string]string // Templated, merged with parent
}

type PackageSource struct {
	Type     string
	CurrRef  SourceRef
	Upstream *UpstreamGit
	Git      *git.Repository
	Username string
	Password string
	Path     string // Local absolute path to repo files
	refs     []SourceRef
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
		if err := p.Packages.Validate(); err != nil { // Recursively validate packages
			return err
		}
	}
	return nil
}

func (fleet *Fleet) Validate() error {
	var names []string
	for idx := range fleet.Spec.Upstreams {
		u := &fleet.Spec.Upstreams[idx]
		names = append(names, string(u.Name))
		if u.Type == api.PackageUpstreamTypeGit {
			switch u.Git.AuthMethod {
			case "":
			case "sshAgent":
				if u.Git.Auth != nil {
					return fmt.Errorf("upstream %v, cannot use auth specification with method 'sshAgent'", u.Name)
				}
			case "sshPrivateKey":
				if u.Git.Auth == nil {
					return fmt.Errorf("upstream %v, auth method 'sshPrivateKey' require auth specification", u.Name)
				}
				if u.Git.Auth.Kind != "Secret" {
					return fmt.Errorf("upstream %v, only auth kind 'Secret' supported", u.Name)
				}
			default:
				return fmt.Errorf("upstream %v, unsupported auth method: %v", u.Name, u.Git.AuthMethod)
			}
		}
	}
	if len(util.UniqueStrings(names)) != len(names) {
		return fmt.Errorf("upstream names must be unique")
	}
	if _, found := fleet.Spec.Defaults.Metadata.Spec["name"]; found {
		return fmt.Errorf("defaults.metadata.spec cannot have 'name' field")
	}
	return fleet.Spec.Packages.Validate()
}

func (fleet *Fleet) Default(packages PackageSlice, parentMeta Metadata) {
	defaults := fleet.Spec.Defaults
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
		if len(p.Metadata.Spec) == 0 {
			p.Metadata.Spec = map[string]string{}
		}
		if _, found := p.Metadata.Spec["name"]; !found {
			p.Metadata.Spec["name"] = p.Name
		}
		if p.Metadata.InheritFromParent == nil {
			p.Metadata.InheritFromParent = PtrTo(true)
		}
		if *p.Metadata.InheritFromParent {
			p.Metadata.mergedSpec = util.MergeMaps(parentMeta.mergedSpec, p.Metadata.Spec)
			p.Metadata.mergedTemplated = util.MergeMaps(parentMeta.mergedTemplated, p.Metadata.Templated)
		} else {
			p.Metadata.mergedSpec = p.Metadata.Spec
			p.Metadata.mergedTemplated = p.Metadata.Templated
		}
		p.dstRelPath = p.Name
		fleet.Default(p.Packages, p.Metadata) // Recursively default packages
	}
}

func (packages PackageSlice) Print(w io.Writer) {
	for idx := range packages {
		p := &packages[idx]
		fmt.Fprintf(w, "%v: %v -> %v\n", p.Name, p.SrcPath, p.dstRelPath)
		p.Packages.Print(w)
	}
}

func (p *Package) renderTemplateMeta(src *PackageSource, dstPath string) error {
	data := map[string]string{
		"name":    p.Name,
		"commit":  src.Git.CurrentHash,
		"rev":     src.Git.CurrentRevision,
		"srcPath": p.SrcPath,
		"dstPath": dstPath,
	}
	pCtx := template.New("tpl").Option("missingkey=error").Funcs(sprig.TxtFuncMap())
	for k, v := range p.Metadata.mergedTemplated {
		var tp bytes.Buffer
		tpl, err := pCtx.Parse(v)
		if err != nil {
			return err
		}
		err = tpl.Execute(&tp, data)
		if err != nil {
			return err
		}
		p.Metadata.mergedSpec[k] = tp.String()
	}
	return nil
}

func ParseFleetSpec(object []byte) (*Fleet, error) {
	fleet := &Fleet{}
	if err := yaml.Unmarshal(object, fleet); err != nil {
		return nil, err
	}

	// Defaults for defaults
	if fleet.Spec.Defaults.Enabled == nil {
		fleet.Spec.Defaults.Enabled = PtrTo(true)
	}
	if fleet.Spec.Defaults.Upstream == "" && len(fleet.Spec.Upstreams) == 1 {
		fleet.Spec.Defaults.Upstream = fleet.Spec.Upstreams[0].Name
	}

	fleet.Spec.Defaults.Metadata.mergedSpec = fleet.Spec.Defaults.Metadata.Spec
	fleet.Spec.Defaults.Metadata.mergedTemplated = fleet.Spec.Defaults.Metadata.Templated
	fleet.Default(fleet.Spec.Packages, fleet.Spec.Defaults.Metadata)
	err := fleet.Validate()
	if err != nil {
		return nil, err
	}

	return fleet, nil
}

func NewPackageSource(u *Upstream, fileBase, username, password string) (*PackageSource, fn.Results, error) {
	var fnResults fn.Results
	if u.Type == api.PackageUpstreamTypeGit {
		// Hash repo url and auth method to create local tmp path
		repoHash := base64.StdEncoding.EncodeToString([]byte(u.Git.Repo + "+" + u.Git.AuthMethod))
		localPath := filepath.Join(fileBase, repoHash)
		start := time.Now()
		r, err := git.Clone(u.Git.Repo, u.Git.AuthMethod, username, password, localPath)
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
			Username: username,
			Password: password,
			Path:     localPath,
			refs:     []SourceRef{}}, fnResults, nil
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
		hash, err := src.Git.Checkout(string(ref))
		if err != nil {
			fnResults = append(fnResults, fn.GeneralResult(fmt.Sprintf("error fetching %v@%v\n", src.Upstream.Repo, ref), fn.Error))
			return fnResults, err
		}
		t := time.Now()
		elapsed := t.Sub(start).Truncate(time.Millisecond)
		fnResults = append(fnResults, fn.GeneralResult(fmt.Sprintf("fetched %v@%v (%v) in %v\n", src.Upstream.Repo, ref, hash, elapsed), fn.Info))
		src.CurrRef = ref
		return fnResults, nil
	}
	return fnResults, nil
}

// TossFiles copies package files
func (fleet *Fleet) TossFiles(sources []PackageSource, packages PackageSlice, dstBaseDir, pkgsBasePath string) (fn.Results, error) {
	var fnResults fn.Results

	fleet.ComputeReferences(sources, packages)

	outPackages := fleet.CollectOutputPackages(sources, packages, pkgsBasePath)

	for idx := range outPackages {
		p := &outPackages[idx]

		d := filepath.Join(dstBaseDir, p.dstAbsPath)
		u := UpstreamLookup(fleet, p.Upstream)
		if u == nil {
			return fnResults, fmt.Errorf("unknown upstream: %v", p.Upstream)
		}
		src := PackageSourceLookup(sources, u)
		if src == nil {
			return fnResults, fmt.Errorf("unknown upstream source: %v", p.Upstream)
		}
		fnRes, err := SourceEnsureVersion(src, p.Ref)
		fnResults = append(fnResults, fnRes...)
		if err != nil {
			return fnResults, err
		}
		util.ResultPrintf(&fnResults, fn.Info, "package %v; %v --> %v", p.Name, p.SrcPath, p.dstRelPath)
		s := filepath.Join(src.Path, p.SrcPath)
		err = os.CopyFS(d, os.DirFS(s))
		if err != nil {
			return fnResults, fmt.Errorf("copying package %v dir (%v --> %v): %v", p.Name, p.SrcPath, p.dstRelPath, err)
		}
		// TODO: assumes git upstream
		err = p.renderTemplateMeta(src, p.dstAbsPath)
		if err != nil {
			return fnResults, fmt.Errorf("rendering package %v metadata: %v", p.Name, err)
		}
		err = kpt.UpdateKptMetadata(d, p.Name, p.Metadata.mergedSpec, p.SrcPath, src.Git.URI, src.Git.CurrentRevision, src.Git.CurrentHash)
		if err != nil {
			return fnResults, fmt.Errorf("mutating package %v metadata: %v", p.Name, err)
		}
		fnResults = append(fnResults, fnRes...)
	}
	return fnResults, nil
}

// CollectOutputPackages will precompute package paths and return a list of packages that should
// be output, i.e. ignoring stubs and disabled packages
func (fleet *Fleet) CollectOutputPackages(sources []PackageSource, packages PackageSlice, pkgsBasePath string) PackageSlice {
	var outPackages PackageSlice
	for idx := range packages {
		p := &packages[idx]
		p.dstAbsPath = filepath.Join(pkgsBasePath, p.dstRelPath)
		if !*p.Enabled {
			continue
		}
		if !*p.Stub {
			outPackages = append(outPackages, *p)
		}
		pkgs := fleet.CollectOutputPackages(sources, p.Packages, p.dstAbsPath)
		outPackages = append(outPackages, pkgs...)
	}
	return outPackages
}

// ComputeReferences loops through all packages and collect all references used by packages
func (fleet *Fleet) ComputeReferences(sources []PackageSource, packages PackageSlice) error {
	for idx := range packages {
		p := &packages[idx]
		if !*p.Enabled {
			continue
		}
		if !*p.Stub {
			u := UpstreamLookup(fleet, p.Upstream)
			src := PackageSourceLookup(sources, u)
			if !slices.Contains(src.refs, p.Ref) {
				src.refs = append(src.refs, p.Ref)
			}
		}
		err := fleet.ComputeReferences(sources, p.Packages)
		if err != nil {
			return err
		}
	}
	return nil
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
