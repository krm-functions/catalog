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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/michaelvl/krm-functions/pkg/helm"
	t "github.com/michaelvl/krm-functions/pkg/helmspecs"
	"sigs.k8s.io/kustomize/kyaml/kio"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

const annotationURL string = "experimental.helm.sh/"
const annotationShaSum string = annotationURL + "chart-sum"

// We cannot use types from helmspecs due to the additional 'Chart' field
type HelmChart struct {
	Args    t.HelmChartArgs       `json:"chartArgs,omitempty" yaml:"chartArgs,omitempty"`
	Options t.HelmTemplateOptions `json:"templateOptions,omitempty" yaml:"templateOptions,omitempty"`
	Chart   string                `json:"chart,omitempty" yaml:"chart,omitempty"`
}

type RenderHelmChart struct {
	APIVersion string      `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string      `json:"kind,omitempty" yaml:"kind,omitempty"`
	Charts     []HelmChart `json:"helmCharts,omitempty" yaml:"helmCharts,omitempty"`
}

func ParseRenderSpec(b []byte) (*RenderHelmChart, error) {
	spec := &RenderHelmChart{}
	if err := kyaml.Unmarshal(b, spec); err != nil {
		return nil, err
	}
	return spec, nil
}

func Run(rl *fn.ResourceList) (bool, error) {
	var outputs fn.KubeObjects
	// cfg := rl.FunctionConfig
	// parseConfig(cfg)

	for _, kubeObject := range rl.Items {
		if kubeObject.IsGVK("experimental.helm.sh", "", "RenderHelmChart") {
			y := kubeObject.String()
			spec, err := ParseRenderSpec([]byte(y))
			if err != nil {
				return false, err
			}
			for idx := range spec.Charts {
				if spec.Charts[idx].Options.ReleaseName == "" {
					return false, fmt.Errorf("invalid chart spec %s: ReleaseName required, index %d", kubeObject.GetName(), idx)
				}
			}
			for idx := range spec.Charts {
				newobjs, err := spec.Charts[idx].Template()
				if err != nil {
					return false, err
				}
				outputs = append(outputs, newobjs...)
			}
		} else if kubeObject.IsGVK("fn.kpt.dev", "", "RenderHelmChart") {
			y := kubeObject.String()
			spec, err := ParseRenderSpec([]byte(y))
			if err != nil {
				return false, err
			}
			for idx := range spec.Charts {
				chart := &spec.Charts[idx]
				var uname, pword *string
				if chart.Args.Auth != nil {
					uname, pword, err = lookupAuthSecret(chart, rl)
					if err != nil {
						return false, err
					}
				}
				chartData, chartSum, err := chart.SourceChart(uname, pword)
				if err != nil {
					return false, err
				}
				err = kubeObject.SetAPIVersion("experimental.helm.sh/v1alpha1")
				if err != nil {
					return false, err
				}
				chs, found, err := kubeObject.NestedSlice("helmCharts")
				if !found {
					return false, fmt.Errorf("helmCharts key not found in %s", kubeObject.GetName())
				}
				if err != nil {
					return false, err
				}
				err = chs[0].SetNestedField(base64.StdEncoding.EncodeToString(chartData), "chart")
				if err != nil {
					return false, err
				}
				err = kubeObject.SetAnnotation(annotationShaSum, "sha256:"+chartSum)
				if err != nil {
					return false, err
				}
				outputs = append(outputs, kubeObject)
			}
		} else {
			outputs = append(outputs, kubeObject)
		}
	}

	rl.Items = outputs
	return true, nil
}

func lookupAuthSecret(chart *HelmChart, rl *fn.ResourceList) (username, password *string, err error) {
	namespace := chart.Args.Auth.Namespace
	if namespace == "" {
		namespace = "default" // Default according to spec
	}
	for _, k := range rl.Items {
		if !k.IsGVK("v1", "", "Secret") || k.GetName() != chart.Args.Auth.Name {
			continue
		}
		oNamespace := k.GetNamespace()
		if oNamespace == "" {
			oNamespace = "default" // Default according to spec
		}
		if namespace == oNamespace {
			uname, found, err := k.NestedString("data", "username")
			if !found {
				return nil, nil, fmt.Errorf("key 'username' not found in Secret '%s'", chart.Args.Auth.Name)
			}
			if err != nil {
				return nil, nil, err
			}
			pword, found, err := k.NestedString("data", "password")
			if !found {
				return nil, nil, fmt.Errorf("key 'password' not found in Secret '%s'", chart.Args.Auth.Name)
			}
			if err != nil {
				return nil, nil, err
			}
			u, err := base64.StdEncoding.DecodeString(uname)
			if err != nil {
				return nil, nil, err
			}
			uname = string(u)
			p, err := base64.StdEncoding.DecodeString(pword)
			if err != nil {
				return nil, nil, err
			}
			pword = string(p)
			return &uname, &pword, nil
		}
	}
	return nil, nil, fmt.Errorf("auth secret '%s' not found", chart.Args.Auth.Name)
}

func (chart *HelmChart) SourceChart(username, password *string) (chartData []byte, chartSha256Sum string, err error) {
	tmpDir, err := os.MkdirTemp("", "chart-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(tmpDir)

	tarball, chartSum, err := helm.PullChart(chart.Args, tmpDir, username, password)
	if err != nil {
		return nil, "", err
	}
	buf, err := os.ReadFile(filepath.Join(tmpDir, tarball))
	if err != nil {
		return nil, "", err
	}
	return buf, chartSum, err
}

func (chart *HelmChart) Template() (fn.KubeObjects, error) {
	chartfile, err := base64.StdEncoding.DecodeString(chart.Chart)
	if err != nil {
		return nil, err
	}
	tmpDir, err := os.MkdirTemp("", "chart-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpDir)

	gzr, err := gzip.NewReader(bytes.NewReader(chartfile))
	if err != nil {
		return nil, err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)

	// Extract tar archive files
	for {
		hdr, xtErr := tr.Next()
		if xtErr == io.EOF {
			break // End of archive
		} else if xtErr != nil {
			return nil, xtErr
		}
		fname := filepath.Join(tmpDir, hdr.Name)
		fdir := filepath.Dir(fname)
		if hdr.Typeflag == tar.TypeReg {
			// Not all tarfiles have explicit directories, i.e. we always create directories if they do not exist
			if _, fErr := os.Stat(fdir); fErr != nil {
				if mkdErr := os.MkdirAll(fdir, 0o755); mkdErr != nil {
					return nil, mkdErr
				}
			}

			file, fErr := os.OpenFile(fname, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if fErr != nil {
				return nil, fErr
			}
			_, fErr = io.Copy(file, tr)
			file.Close()
			if fErr != nil {
				return nil, fErr
			}
		}
	}

	valuesFile := filepath.Join(tmpDir, "values.yaml")
	err = chart.writeValuesFile(valuesFile)
	if err != nil {
		return nil, err
	}
	args := chart.buildHelmTemplateArgs()
	args = append(args, "--values", valuesFile, filepath.Join(tmpDir, chart.Args.Name))

	helmCtxt := helm.NewRunContext()
	defer helmCtxt.DiscardContext()
	stdout, err := helmCtxt.Run(args...)
	if err != nil {
		return nil, err
	}

	r := &kio.ByteReader{Reader: bytes.NewBufferString(string(stdout)), OmitReaderAnnotations: true}
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
		// The sink function conveniently sets path if none is defined

		// annoVal := fmt.Sprintf("%s/%s/%s_%s.yaml",
		// 	chart.Args.Name, chart.Options.ReleaseName, strings.ToLower(o.GetKind()), o.GetName())
		// currAnno := o.GetAnnotations()
		// if len(currAnno) == 0 {
		// 	currAnno = map[string]string{kioutil.PathAnnotation: annoVal}
		// } else {
		// 	currAnno[kioutil.PathAnnotation] = annoVal
		// }
		// err = o.SetNestedStringMap(currAnno, "metadata", "annotations")
		// if err != nil {
		// 	return nil, err
		// }
		objects = append(objects, o)
	}

	if err != nil {
		return nil, err
	}

	return objects, nil
}

// Write embedded values to a file for passing to Helm
func (chart *HelmChart) writeValuesFile(valuesFilename string) error {
	vals := chart.Options.Values.ValuesInline
	b, err := kyaml.Marshal(vals)
	if err != nil {
		return err
	}
	return os.WriteFile(valuesFilename, b, 0o600)
}

func (chart *HelmChart) buildHelmTemplateArgs() []string {
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

func main() {
	// fmt.Fprintf(os.Stderr, "version: %s\n", version.Version)
	if err := fn.AsMain(fn.ResourceListProcessorFunc(Run)); err != nil {
		os.Exit(1)
	}
}
