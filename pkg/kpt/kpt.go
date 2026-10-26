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
	"os"
	"path/filepath"
	"slices"
	"strings"
	"text/template"

	kptfile "github.com/kptdev/kpt/api/kptfile/v1"
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
	var kf *yaml.RNode
	_, err = os.Stat(kfn)
	if err == nil {
		// Kptfile exists already. Edit it as a node tree, a round-trip through the
		// KptFile struct would drop comments, unknown fields and empty omitempty fields
		kf, err = yaml.ReadFile(kfn)
		if err != nil {
			return fmt.Errorf("reading %v: %v", kfn, err)
		}
	} else {
		kf, err = yaml.FromMap(map[string]any{
			"apiVersion": kptfile.TypeMeta.APIVersion,
			"kind":       kptfile.TypeMeta.Kind,
		})
		if err != nil {
			return err
		}
	}
	// 'kpt kpg get' sets name to the name of the package create (to path) and removes namespace
	if err := kf.SetName(pkgName); err != nil {
		return err
	}
	if err := kf.SetNamespace(""); err != nil {
		return err
	}
	// Set source
	upstream := &kptfile.Upstream{
		Type: kptfile.GitOrigin,
		Git: &kptfile.Git{
			Repo:      gitRepo,
			Directory: "/" + gitDirectory,
			Ref:       gitRev,
		},
		UpdateStrategy: kptfile.ResourceMerge,
	}
	upstreamLock := &kptfile.Locator{
		Type: kptfile.GitOrigin,
		Git: &kptfile.GitLock{
			Repo:      gitRepo,
			Directory: "/" + gitDirectory,
			Ref:       gitRev,
			Commit:    gitHash,
		},
	}
	// Insert in reverse order, both after 'metadata', which gives the same field order as kpt
	if err := setFieldAfter(kf, "upstreamLock", upstreamLock, yaml.MetadataField); err != nil {
		return err
	}
	if err := setFieldAfter(kf, "upstream", upstream, yaml.MetadataField); err != nil {
		return err
	}

	return yaml.WriteFile(kf, kfn)
}

// setFieldAfter sets field to the YAML encoding of value. A new field is inserted
// after the field named 'after', or appended if that field does not exist
func setFieldAfter(rn *yaml.RNode, field string, value any, after string) error {
	b, err := yaml.Marshal(value)
	if err != nil {
		return err
	}
	vn, err := yaml.Parse(string(b))
	if err != nil {
		return err
	}
	if rn.Field(field) != nil {
		return rn.PipeE(yaml.SetField(field, vn))
	}
	content := rn.YNode().Content
	pos := len(content)
	for i := 0; i < len(content)-1; i += 2 {
		if content[i].Value == after {
			pos = i + 2
			break
		}
	}
	kv := []*yaml.Node{yaml.NewScalarRNode(field).YNode(), vn.YNode()}
	rn.YNode().Content = slices.Insert(content, pos, kv...)
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
