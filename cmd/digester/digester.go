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
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/krm-functions/catalog/pkg/api"
	"github.com/krm-functions/catalog/pkg/helm"
	t "github.com/krm-functions/catalog/pkg/helmspecs"
	"github.com/krm-functions/catalog/pkg/version"
	"sigs.k8s.io/kustomize/kyaml/fn/framework"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	containerImagePathFilter     = `.*containers\[\d+\].image$`
	initContainerImagePathFilter = `.*initContainers\[\d+\].image$`
	digesterRegexpPrefix         = `# digester: `
)

type ImageFilter struct {
	// List of images found walking resources
	Images []string

	// Map from image (key) to digest (value)
	Digests map[string]string

	// Regular expressions used to identify images
	PathFilters []*regexp.Regexp
}

func NewImageFilter() *ImageFilter {
	i := &ImageFilter{}
	i.PathFilters = append(i.PathFilters, regexp.MustCompile(containerImagePathFilter), regexp.MustCompile(initContainerImagePathFilter))
	i.Digests = make(map[string]string)
	return i
}

func (i *ImageFilter) Process(resourceList *framework.ResourceList) error {
	results := []*framework.Result{}
	results = append(results, &framework.Result{
		Message: "digester",
	})
	for _, iobj := range resourceList.Items {
		if iobj.GetApiVersion() != api.HelmResourceAPIVersion || iobj.GetKind() != "RenderHelmChart" {
			continue
		}
		y := iobj.MustString()
		spec, err := t.ParseKptSpec([]byte(y))
		if err != nil {
			return err
		}
		for idx := range spec.Charts {
			if spec.Charts[idx].Options.ReleaseName == "" {
				return fmt.Errorf("invalid chart spec %s: ReleaseName required, index %d", iobj.GetName(), idx)
			}
		}
		for idx := range spec.Charts {
			chartTarball, err := base64.StdEncoding.DecodeString(spec.Charts[idx].Chart)
			if err != nil {
				return err
			}
			if len(chartTarball) == 0 {
				return fmt.Errorf("no embedded chart found")
			}
			rendered, err := helm.Template(&spec.Charts[idx], chartTarball)
			if err != nil {
				return err
			}
			objs, err := helm.ParseAsRNodes(rendered)
			if err != nil {
				return err
			}
			imageFilter := NewImageFilter()
			_, err = imageFilter.Filter(objs)
			if err != nil {
				return err
			}
			imageFilter.LookupDigests()
			for _, image := range imageFilter.Images {
				results = append(results, &framework.Result{
					Message:  fmt.Sprintf("image: %v\n", image+"@"+imageFilter.Digests[image]),
					Severity: framework.Info,
				})
			}
			_, err = imageFilter.SetDigests(iobj)
			if err != nil {
				return err
			}
		}
	}
	resourceList.Results = results
	return nil
}

func (i *ImageFilter) Filter(nodes []*yaml.RNode) ([]*yaml.RNode, error) { //nolint:unparam // return value is unused, but we want the common filter prototype
	for idx := range nodes {
		err := Walk(i, nodes[idx], "")
		if err != nil {
			return nil, err
		}
	}
	return nodes, nil
}

func (i *ImageFilter) VisitScalar(node *yaml.RNode, path string) error {
	for idx := range i.PathFilters {
		if i.PathFilters[idx].MatchString(path) {
			i.Images = append(i.Images, yaml.GetValue(node))
			break
		}
	}
	return nil
}

func (i *ImageFilter) LookupDigests() {
	for _, image := range i.Images {
		if strings.Contains(image, "@") {
			continue
		}
		digest, err := crane.Digest(image, crane.WithUserAgent(fmt.Sprintf("digester/%s", version.Version)))
		// We dont fail here if we cannot locate a digest, only if the digest is needed for a patch-back target
		if err == nil {
			i.Digests[image] = digest
		}
	}
}

type ImageDigestSetter struct {
	Digests map[string]string
}

func (i *ImageDigestSetter) VisitScalar(node *yaml.RNode, _ string) error {
	comment := node.YNode().LineComment
	if strings.HasPrefix(comment, digesterRegexpPrefix) {
		re := strings.TrimSpace(strings.TrimPrefix(comment, digesterRegexpPrefix))
		pattern, err := regexp.Compile(re)
		if err != nil {
			return fmt.Errorf("cannot parse regexp: %v: %w", re, err)
		}
		var match string
		for k, v := range i.Digests {
			if pattern.MatchString(k) {
				if match != "" { // We already found a match, so the regexp does not uniquely identify a digest
					return fmt.Errorf("regexp does not identify a unique image: %v", re)
				}
				match = v
				// We dont break such that we can check for unique match
			}
		}
		node.YNode().Value = match
	}
	return nil
}

func (i *ImageFilter) SetDigests(node *yaml.RNode) (*yaml.RNode, error) { //nolint:unparam // return value is unused, but we want the common filter prototype
	setter := &ImageDigestSetter{Digests: i.Digests}
	err := Walk(setter, node, "")
	if err != nil {
		return nil, err
	}
	return node, nil
}
