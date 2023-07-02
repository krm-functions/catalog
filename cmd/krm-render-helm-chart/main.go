package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path"
	t "github.com/michaelvl/helm-upgrader/pkg/helmspecs"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
)

type RenderHelmChart struct {
	ApiVersion string                `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
	Kind       string                `json:"kind,omitempty" yaml:"kind,omitempty"`
	Options    t.HelmTemplateOptions `json:"templateOptions,omitempty" yaml:"templateOptions,omitempty"`
	Chart      string                `json:"chart,omitempty" yaml:"chart,omitempty"`
}

func ParseRenderSpec(b []byte) (*RenderHelmChart, error) {
	spec := &RenderHelmChart{}
	if err := kyaml.Unmarshal(b, spec); err != nil {
		return nil, err
	}
	//if !spec.IsValidSpec() {
	//	return spec, fmt.Errorf("Invalid chart spec: %+v\n", spec)
	//}
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
			err = spec.Generate()
			if err != nil {
				return false, err
			}
		} else {
			outputs = append(outputs, kubeObject)
		}
	}

	rl.Items = outputs
	return true, nil
}

func (spec *RenderHelmChart) Generate() error {
	chartfile, err := base64.StdEncoding.DecodeString(spec.Chart)
	if err != nil {
		return err
	}
	tmpDir, err := os.MkdirTemp("", "chart-")
	if err != nil {
		return err
	}
	fmt.Printf("tempDir %s\n", tmpDir)
	//defer os.RemoveAll(tmpDir)

	gzr, err := gzip.NewReader(bytes.NewReader(chartfile))
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)

	// Extract tar achive files
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		} else if err != nil {
			return err
		}
		fname := path.Join(tmpDir, hdr.Name)
		fdir := path.Dir(fname)
		if hdr.Typeflag ==  tar.TypeReg {
			if _, err := os.Stat(fdir); err != nil {
				if err = os.MkdirAll(fdir, 0755); err != nil {
					return err
				}
			}

			file, err:= os.OpenFile(fname, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(hdr.Mode))
			if err != nil {
				return err
			}
			defer file.Close()
			_, err =io.Copy(file, tr)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	//fmt.Fprintf(os.Stderr, "version: %s\n", version.Version)
	if err := fn.AsMain(fn.ResourceListProcessorFunc(Run)); err != nil {
		os.Exit(1)
	}
}
