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
	"text/template"

	"github.com/GoogleContainerTools/kpt/pkg/kptfile"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

var pkgContextCm = `apiVersion: v1
kind: ConfigMap
metadata:
  name: kptfile.kpt.dev
  annotations:
    config.kubernetes.io/local-config: "true"
data:
  name: {{.Name}}
`

func UpdateKptMetadata(path, pkgName, pkgDirectory, gitRepo, gitRef string) error {
	data := map[string]string{
		"Name": pkgName,
	}
	fname := filepath.Join(path, "package-context.yaml")
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
	// kpt kpg get sets name to the name of the package create (to path) and removes namespace
	kf.Name = pkgName
	kf.Namespace = ""
	fmt.Fprintf(os.Stderr, "kptfile: %+v\n", kf)
	err = WriteKptfile(kfn, kf)
	if err != nil {
		return err
	}

	return nil
}

func writeTemplated(templateString, filename string, data map[string]string) error {
	pkgCtx := template.New("tpl")
	pkgCtx = template.Must(pkgCtx.Parse(templateString))
	fh, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("opening for writing %v: %v", filename, err)
	}
	defer fh.Close()
	pkgCtx.Execute(fh, data)

	return nil
}

func ReadKptfile(filename string) (*kptfile.KptFile, error) {
	fmt.Fprintf(os.Stderr, "kptfile read: %v\n", filename)
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
	if err := yaml.Unmarshal([]byte(data), kf); err != nil {
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
