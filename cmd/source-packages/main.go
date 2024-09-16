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
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
	"github.com/krm-functions/catalog/pkg/api"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
)

func Run(rl *fn.ResourceList) (bool, error) {
	results := &rl.Results

	var srcBase, dstBase string
	base := os.Getenv("LOCAL_PACKAGES_DIR")
	if base == "" {
		base = "/tmp/source-packages"
	}
	srcBase = base + "/in"
	dstBase = base + "/out"

	for _, kubeObject := range rl.Items {
		if kubeObject.IsGVK(api.KptResourceAPI, "", "Fleet") {
			object := kubeObject.String()
			objPath := filepath.Join(filepath.Dir(kubeObject.GetAnnotation(kioutil.PathAnnotation)), kubeObject.GetName())
			packages, err := ParsePkgSpec([]byte(object), objPath)
			if err != nil {
				return false, err
			}
			sources, err := packages.FetchSources(srcBase)
			if err != nil {
				return false, err
			}
			for _, src := range sources {
				if src.Type == "git" {
					*results = append(*results, fn.GeneralResult(fmt.Sprintf("Found source %v", src.Upstream.Git.Repo), fn.Info))
				}
			}
			fnResults, err := packages.Spec.Packages.TossFiles(sources, srcBase, filepath.Join(dstBase, kubeObject.GetName()))
			if err != nil {
				return false, err
			}
			*results = append(*results, fnResults...)
			nodes, err := FilesystemToObjects(dstBase)
			if err != nil {
				return false, err
			}
			for _, nn := range nodes {
				err = rl.UpsertObjectToItems(nn, nil, false)
				if err != nil {
					return false, err
				}
			}
		}
	}

	return true, nil
}

func main() {
	if err := fn.AsMain(fn.ResourceListProcessorFunc(Run)); err != nil {
		os.Exit(1)
	}
}
