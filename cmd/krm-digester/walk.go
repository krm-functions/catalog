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

type Visitor interface {
	VisitScalar(node *yaml.RNode, path string) error
}

// Walk visits all nodes in the RNode through recursive traversal
func Walk(v Visitor, object *yaml.RNode, path string) error {
	switch object.YNode().Kind {
	case yaml.DocumentNode:
		return fmt.Errorf("did not expect DocumentNode")
	case yaml.MappingNode:
		return object.VisitFields(func(node *yaml.MapNode) error {
			return Walk(v, node.Value, path+"."+node.Key.YNode().Value)
		})
	case yaml.SequenceNode:
		elements, err := object.Elements()
		if err != nil {
			return err
		}
		for idx := range elements {
			if err := Walk(v, elements[idx], path+fmt.Sprintf("[%d]", idx)); err != nil {
				return fmt.Errorf("waling sequence: %w", err)
			}
		}
	case yaml.ScalarNode:
		err := v.VisitScalar(object, path)
		if err != nil {
			return fmt.Errorf("visiting scalar: %w", err)
		}
	}
	return nil
}
