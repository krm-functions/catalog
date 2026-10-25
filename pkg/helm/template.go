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

package helm

import (
	"bytes"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	t "github.com/krm-functions/catalog/pkg/helmspecs"
	"helm.sh/helm/v4/pkg/action"
	helmchart "helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/loader"
	release "helm.sh/helm/v4/pkg/release/v1"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

// Template renders a chart tarball using given values, equivalent to
// `helm template`. The raw chart tarball data is given in
// `chartTarball` (note, not base64 encoded). Returns the rendered
// text
func Template(chart *t.HelmChart, chartTarball []byte) ([]byte, error) {
	chrt, err := loadChart(chartTarball)
	if err != nil {
		return nil, err
	}

	vals, err := chartValues(chart)
	if err != nil {
		return nil, fmt.Errorf("parsing values: %w", err)
	}

	client, err := newTemplateInstall(chart)
	if err != nil {
		return nil, err
	}

	ri, err := client.Run(chrt, vals)
	if err != nil {
		return nil, fmt.Errorf("rendering chart: %w", err)
	}
	rel, ok := ri.(*release.Release)
	if !ok {
		return nil, fmt.Errorf("unsupported release type: %T", ri)
	}

	// Same output format as `helm template`, i.e. the manifest followed by hooks
	var manifests bytes.Buffer
	fmt.Fprintln(&manifests, strings.TrimSpace(rel.Manifest))
	for _, h := range rel.Hooks {
		if chart.Options.SkipTests && slices.Contains(h.Events, release.HookTest) {
			continue
		}
		fmt.Fprintf(&manifests, "---\n# Source: %s\n%s\n", h.Path, h.Manifest)
	}
	return manifests.Bytes(), nil
}

// newTemplateInstall prepares a client-only install action, configured like `helm template`
func newTemplateInstall(chart *t.HelmChart) (*action.Install, error) {
	opts := chart.Options
	client := action.NewInstall(action.NewConfiguration())
	client.DryRunStrategy = action.DryRunClient
	client.Replace = true // Skip the name check
	client.ReleaseName = "release-name"
	client.NameTemplate = opts.NameTemplate
	client.Description = opts.Description
	client.IncludeCRDs = opts.IncludeCRDs
	client.APIVersions = common.VersionSet(opts.APIVersions)
	client.Namespace = opts.Namespace
	if client.Namespace == "" {
		client.Namespace = "default"
	}
	if opts.KubeVersion != "" {
		kubeVersion, err := common.ParseKubeVersion(opts.KubeVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid kube version %q: %w", opts.KubeVersion, err)
		}
		client.KubeVersion = kubeVersion
	}

	// Release name resolution as `helm template [NAME] CHART`
	args := []string{chart.Args.Name}
	if opts.ReleaseName != "" {
		args = []string{opts.ReleaseName, chart.Args.Name}
	}
	name, _, err := client.NameAndChart(args)
	if err != nil {
		return nil, err
	}
	client.ReleaseName = name
	return client, nil
}

// loadChart loads a chart tarball and rejects charts `helm template` would not render
func loadChart(chartTarball []byte) (helmchart.Charter, error) {
	chrt, err := loader.LoadArchive(bytes.NewReader(chartTarball))
	if err != nil {
		return nil, fmt.Errorf("loading chart: %w", err)
	}
	ac, err := helmchart.NewDefaultAccessor(chrt)
	if err != nil {
		return nil, err
	}
	switch typ := ac.MetadataAsMap()["Type"]; typ {
	case nil, "", "application":
	default:
		return nil, fmt.Errorf("%s charts are not installable", typ)
	}
	if req := ac.MetaDependencies(); len(req) > 0 {
		if err = action.CheckDependencies(chrt, req); err != nil {
			return nil, errors.Join(errors.New("chart dependencies missing from chart tarball"), err)
		}
	}
	return chrt, nil
}

// chartValues returns the inline chart values. Values are round-tripped
// through YAML to get the same value types as when Helm reads a values file
func chartValues(chart *t.HelmChart) (map[string]any, error) {
	b, err := kyaml.Marshal(chart.Options.Values.ValuesInline)
	if err != nil {
		return nil, err
	}
	return common.ReadValues(b)
}

func ParseAsKubeObjects(rendered []byte) (fn.KubeObjects, error) {
	r := &kio.ByteReader{Reader: bytes.NewBufferString(string(rendered)), OmitReaderAnnotations: true}
	nodes, err := r.Read()
	if err != nil {
		return nil, err
	}

	var objects fn.KubeObjects
	for i := range nodes {
		o, parseErr := fn.ParseKubeObject([]byte(nodes[i].MustString()))
		if parseErr != nil {
			if strings.Contains(parseErr.Error(), "expected exactly one object, got 0") {
				continue
			}
			return nil, fmt.Errorf("failed to parse %s: %s", nodes[i].MustString(), parseErr.Error())
		}
		objects = append(objects, o)
	}

	if err != nil {
		return nil, err
	}

	return objects, nil
}

func ParseAsRNodes(rendered []byte) ([]*kyaml.RNode, error) {
	r := &kio.ByteReader{Reader: bytes.NewBufferString(string(rendered)), OmitReaderAnnotations: true}
	nodes, err := r.Read()
	if err != nil {
		return nil, err
	}
	return nodes, nil
}
