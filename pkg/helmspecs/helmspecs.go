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

package helmspecs

import (
	"fmt"

	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

// Kpt Helm related types
type HelmChart struct {
	Args    HelmChartArgs       `json:"chartArgs,omitempty" yaml:"chartArgs,omitempty"`
	Options HelmTemplateOptions `json:"templateOptions,omitempty" yaml:"templateOptions,omitempty"`
	// This is an extension field from api version 'experimental.helm.sh/v1alpha1'
	Chart string `json:"chart,omitempty" yaml:"chart,omitempty"`
}
type HelmChartArgs struct {
	Name     string                    `json:"name,omitempty" yaml:"name,omitempty"`
	Version  string                    `json:"version,omitempty" yaml:"version,omitempty"`
	Repo     string                    `json:"repo,omitempty" yaml:"repo,omitempty"`
	Registry string                    `json:"registry,omitempty" yaml:"registry,omitempty"`
	Auth     *kyaml.ResourceIdentifier `json:"auth,omitempty" yaml:"auth,omitempty"`
}
type HelmTemplateOptions struct {
	APIVersions  []string   `json:"apiVersions,omitempty" yaml:"apiVersions,omitempty"`
	ReleaseName  string     `json:"releaseName,omitempty" yaml:"releaseName,omitempty"`
	Namespace    string     `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Description  string     `json:"description,omitempty" yaml:"description,omitempty"`
	NameTemplate string     `json:"nameTemplate,omitempty" yaml:"nameTemplate,omitempty"`
	IncludeCRDs  bool       `json:"includeCRDs,omitempty" yaml:"includeCRDs,omitempty"`
	SkipTests    bool       `json:"skipTests,omitempty" yaml:"skipTests,omitempty"`
	Values       HelmValues `json:"values,omitempty" yaml:"values,omitempty"`
}
type HelmValues struct {
	ValuesFiles  []string       `json:"valuesFiles,omitempty" yaml:"valuesFiles,omitempty"`
	ValuesInline map[string]any `json:"valuesInline,omitempty" yaml:"valuesInline,omitempty"`
	ValuesMerge  string         `json:"valuesMerge,omitempty" yaml:"valuesMerge,omitempty"`
}

// https://catalog.kpt.dev/render-helm-chart/v0.2/
type RenderHelmChart struct {
	Kind   string      `json:"kind,omitempty" yaml:"kind,omitempty"`
	Charts []HelmChart `json:"helmCharts,omitempty" yaml:"helmCharts,omitempty"`
}

// ArgoCD Helm related types
type ArgoCDHelmSource struct {
	Name    string `json:"chart,omitempty" yaml:"chart,omitempty"`
	Version string `json:"targetRevision,omitempty" yaml:"targetRevision,omitempty"`
	Repo    string `json:"repoURL,omitempty" yaml:"repoURL,omitempty"`
}
type ArgoCDHelmSpec struct {
	Source ArgoCDHelmSource `json:"source,omitempty" yaml:"source,omitempty"`
}
type ArgoCDHelmApp struct {
	Kind string         `json:"kind,omitempty" yaml:"kind,omitempty"`
	Spec ArgoCDHelmSpec `json:"spec,omitempty" yaml:"spec,omitempty"`
}

func ParseKptSpec(b []byte) (*RenderHelmChart, error) {
	spec := &RenderHelmChart{}
	if err := kyaml.Unmarshal(b, spec); err != nil {
		return nil, err
	}
	if err := spec.IsValidSpec(); err != nil {
		return spec, err
	}
	return spec, nil
}

func (spec *RenderHelmChart) IsValidSpec() error {
	if spec.Kind != "RenderHelmChart" {
		return fmt.Errorf("unsupported kind: %s", spec.Kind)
	}
	for idx := range spec.Charts {
		chart := &spec.Charts[idx]
		if chart.Args.Name == "" || chart.Args.Version == "" || chart.Args.Repo == "" {
			return fmt.Errorf("chart name, version or repo cannot be empty (%s,%s,%s)",
				chart.Args.Name, chart.Args.Version, chart.Args.Repo)
		}
		if chart.Args.Auth != nil {
			if chart.Args.Auth.Kind != "Secret" {
				return fmt.Errorf("chart auth kind must be 'Secret'")
			}
			if chart.Args.Auth.Name == "" {
				return fmt.Errorf("chart auth name must be defined")
			}
		}
	}
	return nil
}

func ParseArgoCDSpec(b []byte) (*ArgoCDHelmApp, error) {
	app := &ArgoCDHelmApp{}
	if err := kyaml.Unmarshal(b, app); err != nil {
		return nil, err
	}
	if !app.IsValidSpec() {
		return app, fmt.Errorf("invalid chart spec: %+v", app)
	}
	return app, nil
}

func (app *ArgoCDHelmApp) IsValidSpec() bool {
	if app.Kind != "Application" {
		return false
	}
	if app.Spec.Source.Name == "" || app.Spec.Source.Version == "" || app.Spec.Source.Repo == "" {
		return false
	}
	return true
}

func (asrc *ArgoCDHelmSource) ToKptSpec() HelmChartArgs {
	ksrc := HelmChartArgs{}
	ksrc.Name = asrc.Name
	ksrc.Version = asrc.Version
	ksrc.Repo = asrc.Repo
	return ksrc
}
