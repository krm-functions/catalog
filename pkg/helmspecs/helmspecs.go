package helmspecs

import (
	"fmt"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

// Kpt Helm related types
type HelmChart struct {
	Args    HelmChartArgs       `json:"chartArgs,omitempty" yaml:"chartArgs,omitempty"`
	Options HelmTemplateOptions `json:"templateOptions,omitempty" yaml:"templateOptions,omitempty"`
}
type HelmChartArgs struct {
	Name    string `json:"name,omitempty" yaml:"name,omitempty"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	Repo    string `json:"repo,omitempty" yaml:"repo,omitempty"`
}
type HelmTemplateOptions struct {
	ApiVersions []string `json:"apiVersions,omitempty" yaml:"apiVersions,omitempty"`
	ReleaseName string   `json:"releaseName,omitempty" yaml:"releaseName,omitempty"`
	Namespace string     `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	NameTemplate string  `json:"nameTemplate,omitempty" yaml:"nameTemplate,omitempty"`
	IncludeCRDs bool     `json:"includeCRDs,omitempty" yaml:"includeCRDs,omitempty"`
	SkipTests bool       `json:"skipTests,omitempty" yaml:"skipTests,omitempty"`
	Values HelmValues    `json:"values,omitempty" yaml:"values,omitempty"`
}
type HelmValues struct {
	ValuesFiles []string `json:"valuesFiles,omitempty" yaml:"valuesFiles,omitempty"`
	ValuesInline map[string]any `json:"valuesInline,omitempty" yaml:"valuesInline,omitempty"`
	ValuesMerge string    `json:"valuesMerge,omitempty" yaml:"valuesMerge,omitempty"`
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
	if !spec.IsValidSpec() {
		return spec, fmt.Errorf("Invalid chart spec: %+v\n", spec)
	}
	return spec, nil
}

func (spec *RenderHelmChart) IsValidSpec() bool {
	if spec.Kind != "RenderHelmChart" {
		return false
	}
	for _, chart := range spec.Charts {
		if chart.Args.Name == "" || chart.Args.Version == "" || chart.Args.Repo == "" {
			return false
		}
	}
	return true
}

func ParseArgoCDSpec(b []byte) (*ArgoCDHelmApp, error) {
	app := &ArgoCDHelmApp{}
	if err := kyaml.Unmarshal(b, app); err != nil {
		return nil, err
	}
	if !app.IsValidSpec() {
		return app, fmt.Errorf("Invalid chart spec: %+v\n", app)
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
