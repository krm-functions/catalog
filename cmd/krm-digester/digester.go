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
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/crane"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	containerImagePathFilter     = `.*containers\[\d+\].image$`
	initContainerImagePathFilter = `.*initContainers\[\d+\].image$`
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
			i.Images = append(i.Images, node.YNode().Value)
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
		digest, err := crane.Digest(image)
		// We dont fail here if we cannot locate a digest, only if the digest is needed for a patch-back target
		if err == nil {
			i.Digests[image] = digest
		}
	}
}
