// Copyright 2023 Michael Vittrup Larsen

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"

	"github.com/GoogleContainerTools/kpt-functions-catalog/functions/go/apply-setters/applysetters"
	"sigs.k8s.io/kustomize/kyaml/fn/framework"
	"sigs.k8s.io/kustomize/kyaml/fn/framework/command"
	"sigs.k8s.io/kustomize/kyaml/resid"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	ktypes "sigs.k8s.io/kustomize/api/types"
)

const (
	configApiVersion = "experimental.fn.kpt.dev/v1alpha1"
	configKind       = "ApplySetters"
)

func main() {
	asp := ApplySettersProcessor{}
	cmd := command.Build(&asp, command.StandaloneEnabled, false)

	//cmd.Short = generated.ApplySettersShort
	//cmd.Long = generated.ApplySettersLong
	//cmd.Example = generated.ApplySettersExamples

	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type ApplySettersProcessor struct{}

func (asp *ApplySettersProcessor) Process(rl *framework.ResourceList) error {
	var results framework.Results

	results = append(results, &framework.Result{
		Message: "apply-setters",
		Severity: framework.Info,
	})

	setters, err := getSetters(rl)
	if err != nil {
		results = append(results, &framework.Result{
			Message: "no setters definitions found",
		})
		return nil
	}

	_, err = setters.Filter(rl.Items)
	if err != nil {
		results = append(results, &framework.Result{
			Message:  fmt.Sprintf("failed to apply setters: %s", err.Error()),
			Severity: framework.Error,
		})
		return err
	}

	if len(setters.Results) == 0 {
		results = append(results, &framework.Result{
			Message: "no matches for input setter(s)",
		})
	} else {
		for _, res := range setters.Results {
			results = append(results, &framework.Result{
				Message: fmt.Sprintf("set field value to %q", res.Value),
				Field:   &framework.Field{Path: res.FieldPath},
				File:    &framework.File{Path: res.FilePath},
			})
		}
	}

	rl.Results = results
	return nil
}

// getDataMap is called with an ApplySetters resource and return setters as defined by 'data'
func getDataMap(rn *kyaml.RNode) map[string]string {
	n, err := rn.Pipe(kyaml.Lookup("setters", "data"))
	if err != nil {
		return nil
	}
	result := map[string]string{}
	_ = n.VisitFields(func(node *kyaml.MapNode) error {
		result[kyaml.GetValue(node.Key)] = kyaml.GetValue(node.Value)
		return nil
	})
	return result
}

// getReferenceSetters is called with an ApplySetters resource and return setters as defined by 'references'
func getReferenceSetters(rn *kyaml.RNode, resources []*kyaml.RNode) map[string]string {
	n, err := rn.Pipe(kyaml.Lookup("setters", "references"))
	if err != nil {
		return nil
	}
	result := map[string]string{}
	_ = n.VisitElements(func(node *kyaml.RNode) error {
		source := &ktypes.SourceSelector{}

		val, err := node.GetString("source.kind")
		if err != nil {
			return err
		}
		source.ResId.Gvk.Kind = val
		val, err = node.GetString("source.name")
		if err != nil {
			return err
		}
		source.ResId.Name = val
		// TODO: More fields

		val, err = node.GetString("source.fieldPath")
		if err != nil {
			return err
		}
		source.FieldPath = val

		setterValue, err := lookFieldPathSetter(source, resources)
		if err != nil {
			return err
		}
		asSetter, err := node.GetString("as")
		if err != nil {
			return err
		}
		result[asSetter] = setterValue
		return nil
	})
	return result
}

func lookFieldPathSetter(source *ktypes.SourceSelector, resources []*kyaml.RNode) (string, error) {
	var selected []*kyaml.RNode
	for _, rn := range resources {
		resId := resid.FromRNode(rn)
		if resId.IsSelectedBy(source.ResId) {
			selected = append(selected, rn)
		}
	}
	if len (selected) == 0 {
		return "", fmt.Errorf("Nothing matched by %+v", source)
	}
	if len (selected) > 1 {
		return "", fmt.Errorf("Multiple resources (%v) match %+v", len(selected), source)
	}
	val, err := selected[0].GetFieldValue(source.FieldPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", val), nil
}

func getSetters(rl *framework.ResourceList) (applysetters.ApplySetters, error) {
	var setters applysetters.ApplySetters

	// Standard setters from function-config, ConfigMap-style
	applysetters.Decode(rl.FunctionConfig, &setters)

	for _, rn := range rl.Items {
		if rn.GetKind() == configKind && rn.GetApiVersion() == configApiVersion {
			for k, v := range getDataMap(rn) {
				setters.Setters = append(setters.Setters, applysetters.Setter{Name: k, Value: v})
			}
			for k, v := range getReferenceSetters(rn, rl.Items) {
				setters.Setters = append(setters.Setters, applysetters.Setter{Name: k, Value: v})
			}
		}
	}

	return setters, nil
}
