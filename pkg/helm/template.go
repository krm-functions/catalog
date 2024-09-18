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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	securejoin "github.com/cyphar/filepath-securejoin"
	t "github.com/krm-functions/catalog/pkg/helmspecs"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	maxChartTemplateFileLength = 1024 * 1024
)

// Template extracts a chart tarball and renders the chart using given
// values and `helm template`. The raw chart tarball data is given in
// `chartTarball` (note, not base64 encoded). Returns the rendered
// text
func Template(chart *t.HelmChart, chartTarball []byte) ([]byte, error) {
	tmpDir, err := os.MkdirTemp("", "chart-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	err = ExtractChart(chartTarball, tmpDir)
	if err != nil {
		return nil, fmt.Errorf("extracting chart: %w", err)
	}

	valuesFile := filepath.Join(tmpDir, "values.yaml")
	err = writeValuesFile(chart, valuesFile)
	if err != nil {
		return nil, fmt.Errorf("writing values file: %w", err)
	}
	args := buildHelmTemplateArgs(chart)
	args = append(args, "--values", valuesFile, filepath.Join(tmpDir, chart.Args.Name))

	helmCtxt := NewRunContext()
	defer helmCtxt.DiscardContext()
	stdout, err := helmCtxt.Run(args...)
	if err != nil {
		return nil, fmt.Errorf("running helm template: %w", err)
	}

	return stdout, nil
}

// ExtractChart extracts a chart tarball into destDir
func ExtractChart(chartTarball []byte, destDir string) error {
	gzr, err := gzip.NewReader(bytes.NewReader(chartTarball))
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)

	// Extract tar archive files
	for {
		hdr, xtErr := tr.Next()
		if xtErr == io.EOF {
			break // End of archive
		} else if xtErr != nil {
			return xtErr
		}
		fname := hdr.Name
		if path.IsAbs(fname) || strings.Contains(fname, "..") {
			return errors.New("chart contains file with illegal path")
		}
		fileWithPath, fnerr := securejoin.SecureJoin(destDir, fname)
		if fnerr != nil {
			return fnerr
		}
		if hdr.Typeflag == tar.TypeReg {
			fdir := filepath.Dir(fileWithPath)
			if mkdErr := os.MkdirAll(fdir, 0o755); mkdErr != nil {
				return mkdErr
			}

			file, fErr := os.Create(fileWithPath)

			if fErr != nil {
				return fErr
			}
			_, fErr = io.CopyN(file, tr, maxChartTemplateFileLength)
			file.Close()
			if fErr != nil && fErr != io.EOF {
				return fErr
			}
		}
	}
	return nil
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

// writeValuesFile writes chart values to a file for passing to Helm
func writeValuesFile(chart *t.HelmChart, valuesFilename string) error {
	vals := chart.Options.Values.ValuesInline
	b, err := kyaml.Marshal(vals)
	if err != nil {
		return err
	}
	return os.WriteFile(valuesFilename, b, 0o600)
}

// buildHelmTemplateArgs prepares arguments for `helm template`
func buildHelmTemplateArgs(chart *t.HelmChart) []string {
	opts := chart.Options
	args := []string{"template"}
	if opts.ReleaseName != "" {
		args = append(args, opts.ReleaseName)
	}
	if opts.Namespace != "" {
		args = append(args, "--namespace", opts.Namespace)
	}
	if opts.NameTemplate != "" {
		args = append(args, "--name-template", opts.NameTemplate)
	}
	for _, apiVer := range opts.APIVersions {
		args = append(args, "--api-versions", apiVer)
	}
	if opts.KubeVersion != "" {
		args = append(args, "--kube-version", opts.KubeVersion)
	}
	if opts.Description != "" {
		args = append(args, "--description", opts.Description)
	}
	if opts.IncludeCRDs {
		args = append(args, "--include-crds")
	}
	if opts.SkipTests {
		args = append(args, "--skip-tests")
	}
	return args
}
