// Copyright 2023 Michael Vittrup Larsen
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
	"errors"
	"fmt"
	"os"

	"github.com/GoogleContainerTools/kpt-functions-catalog/functions/go/apply-setters/applysetters"
	ktypes "sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/fn/framework"
	"sigs.k8s.io/kustomize/kyaml/fn/framework/command"
	"sigs.k8s.io/kustomize/kyaml/resid"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	configAPIVersion = "fn.kpt.dev/v1alpha1"
	configKind       = "ApplySetters"
)

func main() {
	asp := ApplySettersProcessor{}
	cmd := command.Build(&asp, command.StandaloneEnabled, false)

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

type ApplySettersProcessor struct{}

func (asp *ApplySettersProcessor) Process(rl *framework.ResourceList) error {
	var results framework.Results

	results = append(results, &framework.Result{
		Message:  "apply-setters",
		Severity: framework.Info,
	})

	setters := getSetters(rl)

	_, err := setters.Filter(rl.Items)
	if err != nil {
		results = append(results, &framework.Result{
			Message:  fmt.Sprintf("failed to apply setters: %s", err.Error()),
			Severity: framework.Error,
		})
		rl.Results = results
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
		notFound := &kyaml.NoFieldError{}

		getOrErr := func(path string, valueDest *string) error {
			val, err := node.GetString(path)
			if err != nil {
				if errors.As(err, notFound) {
					return nil
				}
				return err
			}
			*valueDest = val
			return nil
		}

		if err := getOrErr("source.group", &source.ResId.Gvk.Group); err != nil {
			return err
		}
		if err := getOrErr("source.version", &source.ResId.Gvk.Version); err != nil {
			return err
		}
		if err := getOrErr("source.kind", &source.ResId.Gvk.Kind); err != nil {
			return err
		}
		if err := getOrErr("source.name", &source.ResId.Name); err != nil {
			return err
		}
		if err := getOrErr("source.namespace", &source.ResId.Namespace); err != nil {
			return err
		}
		if err := getOrErr("source.fieldPath", &source.FieldPath); err != nil {
			return err
		}

		setterValue, err := lookupFieldPathSetter(source, resources)
		if err != nil {
			return err
		}
		asSetter, err := node.GetString("setterName")
		if err != nil {
			return err
		}
		result[asSetter] = setterValue
		return nil
	})
	return result
}

func lookupFieldPathSetter(source *ktypes.SourceSelector, resources []*kyaml.RNode) (string, error) {
	var selected []*kyaml.RNode
	for _, rn := range resources {
		resID := resid.FromRNode(rn)
		if resID.IsSelectedBy(source.ResId) {
			selected = append(selected, rn)
		}
	}
	if len(selected) == 0 {
		return "", fmt.Errorf("nothing matched by %+v", source)
	}
	if len(selected) > 1 {
		return "", fmt.Errorf("multiple resources (%v) match %+v", len(selected), source)
	}
	val, err := selected[0].GetFieldValue(source.FieldPath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", val), nil
}

func getSetters(rl *framework.ResourceList) applysetters.ApplySetters {
	var setters applysetters.ApplySetters

	// Standard setters from function-config, ConfigMap-style
	fnCfg := rl.FunctionConfig
	if fnCfg.GetKind() == "ConfigMap" {
		applysetters.Decode(fnCfg, &setters)
	} else if fnCfg.GetKind() == configKind && fnCfg.GetApiVersion() == configAPIVersion {
		for k, v := range getDataMap(fnCfg) {
			setters.Setters = append(setters.Setters, applysetters.Setter{Name: k, Value: v})
		}
		for k, v := range getReferenceSetters(fnCfg, rl.Items) {
			setters.Setters = append(setters.Setters, applysetters.Setter{Name: k, Value: v})
		}
	}

	for _, rn := range rl.Items {
		if rn.GetKind() == configKind && rn.GetApiVersion() == configAPIVersion {
			for k, v := range getDataMap(rn) {
				setters.Setters = append(setters.Setters, applysetters.Setter{Name: k, Value: v})
			}
			for k, v := range getReferenceSetters(rn, rl.Items) {
				setters.Setters = append(setters.Setters, applysetters.Setter{Name: k, Value: v})
			}
		}
	}

	return setters
}
