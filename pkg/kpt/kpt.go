// Copyright 2024 Michael Vittrup Larsen
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Utility packages for creating kpt compatible packages
package kpt

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	kptfile "github.com/nephio-project/porch/pkg/kpt/api/kptfile/v1"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

var pkgContextCm = `apiVersion: v1
kind: ConfigMap
metadata:
  name: kptfile.kpt.dev
  annotations:
    config.kubernetes.io/local-config: "true"
data:
{{- range $k,$v := .Data }}
  {{$k}}: {{ $v | toYaml | indent 2}}
{{- end }}
`

func UpdateKptMetadata(path, pkgName string, metadata map[string]string, gitDirectory, gitRepo, gitRev, gitHash string) error {
	fname := filepath.Join(path, "package-context.yaml")
	data := map[string]any{
		"Data": metadata,
	}
	err := writeTemplated(pkgContextCm, fname, data)
	if err != nil {
		return err
	}

	kfn := filepath.Join(path, kptfile.KptFileName)
	var kf *kptfile.KptFile
	_, err = os.Stat(kfn)
	if err == nil {
		// Kptfile exists already
		kf, err = ReadKptfile(kfn)
		if err != nil {
			return err
		}
	} else {
		kf = &kptfile.KptFile{}
		kf.ResourceMeta = kptfile.TypeMeta
	}
	// 'kpt kpg get' sets name to the name of the package create (to path) and removes namespace
	kf.Name = pkgName
	kf.Namespace = ""
	// Set source
	kf.Upstream = &kptfile.Upstream{
		Type: "git",
		Git: &kptfile.Git{
			Repo:      gitRepo,
			Directory: "/" + gitDirectory,
			Ref:       gitRev,
		},
		UpdateStrategy: "resource-merge",
	}
	kf.UpstreamLock = &kptfile.UpstreamLock{
		Type: "git",
		Git: &kptfile.GitLock{
			Repo:      gitRepo,
			Directory: "/" + gitDirectory,
			Ref:       gitRev,
			Commit:    gitHash,
		},
	}

	err = WriteKptfile(kfn, kf)
	if err != nil {
		return err
	}

	return nil
}

func toYAML(v interface{}) string {
	data, err := yaml.Marshal(v)
	if err != nil {
		return ""
	}
	return strings.TrimSuffix(string(data), "\n")
}

func indent(spaces int, v string) string {
	pad := strings.Repeat(" ", spaces)
	return strings.ReplaceAll(v, "\n", "\n"+pad)
}

func writeTemplated(templateString, filename string, data map[string]any) error {
	pkgCtx := template.New("tpl").Funcs(map[string]any{
		"toYaml": toYAML,
		"indent": indent,
	})
	pkgCtx = template.Must(pkgCtx.Parse(templateString))
	fh, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("opening for writing %v: %v", filename, err)
	}
	defer fh.Close()
	return pkgCtx.Execute(fh, data)
}

func ReadKptfile(filename string) (*kptfile.KptFile, error) {
	kr, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("opening %v: %v", filename, err)
	}
	b, err := io.ReadAll(kr)
	if err != nil {
		return nil, fmt.Errorf("reading %v: %v", filename, err)
	}
	return ParseKptfile(b)
}

func ParseKptfile(data []byte) (*kptfile.KptFile, error) {
	kf := &kptfile.KptFile{}
	if err := yaml.Unmarshal(data, kf); err != nil {
		return nil, err
	}
	return kf, nil
}

func WriteKptfile(filename string, kf *kptfile.KptFile) error {
	kw, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("opening for update %v: %v", filename, err)
	}
	defer kw.Close()
	b, err := yaml.Marshal(kf)
	if err != nil {
		return err
	}
	_, err = kw.Write(b)
	if err != nil {
		return fmt.Errorf("writing %v: %v", filename, err)
	}
	return nil
}
