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

func getSetters(rl *framework.ResourceList) (applysetters.ApplySetters, error) {
	var setters applysetters.ApplySetters

	applysetters.Decode(rl.FunctionConfig, &setters)

	return setters, nil
}
