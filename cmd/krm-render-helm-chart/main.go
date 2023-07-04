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
	"github.com/michaelvl/helm-upgrader/pkg/helm"
	t "github.com/michaelvl/helm-upgrader/pkg/helmspecs"
	"sigs.k8s.io/kustomize/kyaml/kio"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
)

const annotationUrl string = "experimental.helm.sh/"
const annotationShaSum string = annotationUrl + "chart-sum"

type HelmChart struct {
	Args       t.HelmChartArgs       `json:"chartArgs,omitempty" yaml:"chartArgs,omitempty"`
	Options    t.HelmTemplateOptions `json:"templateOptions,omitempty" yaml:"templateOptions,omitempty"`
	Chart      string                `json:"chart,omitempty" yaml:"chart,omitempty"`
}

type RenderHelmChart struct {
	ApiVersion string      `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string      `json:"kind,omitempty" yaml:"kind,omitempty"`
	Charts     []HelmChart `json:"helmCharts,omitempty" yaml:"helmCharts,omitempty"`
}

func ParseRenderSpec(b []byte) (*RenderHelmChart, error) {
	spec := &RenderHelmChart{}
	if err := kyaml.Unmarshal(b, spec); err != nil {
		return nil, err
	}
	for idx, chart := range spec.Charts {
		if chart.Options.ReleaseName == "" {
			return nil, fmt.Errorf("Invalid chart spec: ReleaseName required, index %d", idx)
		}
	}
	return spec, nil
}

func Run(rl *fn.ResourceList) (bool, error) {
	var outputs fn.KubeObjects
	//cfg := rl.FunctionConfig
	//parseConfig(cfg)

	for _, kubeObject := range rl.Items {
		if kubeObject.IsGVK("experimental.helm.sh", "", "RenderHelmChart") {
			y := kubeObject.String()
			spec, err := ParseRenderSpec([]byte(y))
			if err != nil {
				return false, err
			}
			for _, chart := range spec.Charts {
				newobjs, err := chart.Template()
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
			for _, chart := range spec.Charts {
				chartData, chartSum, err := chart.SourceChart()
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

func (chart *HelmChart) SourceChart() ([]byte, string, error) {
	tmpDir, err := os.MkdirTemp("", "chart-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(tmpDir)

	tarball, chartSum, err := helm.PullChart(chart.Args, tmpDir)
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

	// Extract tar achive files
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		} else if err != nil {
			return nil, err
		}
		fname := filepath.Join(tmpDir, hdr.Name)
		fdir := filepath.Dir(fname)
		if hdr.Typeflag ==  tar.TypeReg {
			// Not all tarfiles have explicit directories, i.e. we always create directories if they do not exist
			if _, err := os.Stat(fdir); err != nil {
				if err = os.MkdirAll(fdir, 0755); err != nil {
					return nil, err
				}
			}

			file, err:= os.OpenFile(fname, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return nil, err
			}
			defer file.Close()
			_, err =io.Copy(file, tr)
			if err != nil {
				return nil, err
			}
		}
	}

	valuesFile := filepath.Join(tmpDir, "values.yaml")
	err = chart.writeValuesFile(valuesFile)
	if err != nil {
		return nil, err
	}
	args := chart.buildHelmTemplateArgs()
	args = append(args, "--values", valuesFile)
	args = append(args, filepath.Join(tmpDir, chart.Args.Name))

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
		o, err := fn.ParseKubeObject([]byte(nodes[i].MustString()))
		if err != nil {
			if strings.Contains(err.Error(), "expected exactly one object, got 0") {
				continue
			}
			return nil, fmt.Errorf("failed to parse %s: %s", nodes[i].MustString(), err.Error())
		}
		annoVal := fmt.Sprintf("%s/%s/%s_%s.yaml",
			chart.Args.Name, chart.Options.ReleaseName, strings.ToLower(o.GetKind()), o.GetName())
		err = o.SetAnnotation(kioutil.PathAnnotation, annoVal)
		if err != nil {
			return nil, err
		}
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
	return os.WriteFile(valuesFilename, b, 0644)
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
	for _, apiVer := range opts.ApiVersions {
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
	//fmt.Fprintf(os.Stderr, "version: %s\n", version.Version)
	if err := fn.AsMain(fn.ResourceListProcessorFunc(Run)); err != nil {
		os.Exit(1)
	}
}
