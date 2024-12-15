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
	"github.com/krm-functions/catalog/pkg/util"
	"sigs.k8s.io/kustomize/kyaml/kio/kioutil"
)

func Run(rl *fn.ResourceList) (bool, error) {
	results := &rl.Results
	sources := make([]PackageSource, 0)

	var base, srcBase, dstBase string
	base = os.Getenv("LOCAL_PACKAGES_DIR")
	if base == "" {
		var er error
		base, er = os.MkdirTemp("", "package-composer")
		if er != nil {
			return false, er
		}
		defer os.RemoveAll(base)
	}
	srcBase = base + "/in"
	dstBase = base + "/out"

	for _, kubeObject := range rl.Items {
		if !kubeObject.IsGVK(api.KptResourceAPI, "", "Fleet") {
			continue
		}
		object := kubeObject.String()
		fleet, err := ParseFleetSpec([]byte(object))
		if err != nil {
			return false, err
		}
		for idx := range fleet.Spec.Upstreams {
			var er error
			u := &fleet.Spec.Upstreams[idx]
			if PackageSourceLookup(sources, u) != nil {
				continue
			}

			var username, password string
			username = "git"
			if u.Git.Auth != nil {
				username, password, err = util.LookupSSHAuthSecret(u.Git.Auth.Name, u.Git.Auth.Namespace, rl)
				if err != nil {
					return false, err
				}
			}

			src, fnRes, er := NewPackageSource(u, srcBase, username, password)
			if er != nil {
				return false, er
			}
			*results = append(*results, fnRes...)
			sources = append(sources, *src)
		}
		objPath := filepath.Dir(kubeObject.GetAnnotation(kioutil.PathAnnotation))
		fleetBaseDir := filepath.Join(objPath, kubeObject.GetName())
		fnResults, err := fleet.TossFiles(sources, fleet.Spec.Packages, dstBase, fleetBaseDir)
		if err != nil {
			return false, err
		}
		*results = append(*results, fnResults...)
	}
	nodes, err := FilesystemToObjects(dstBase)
	if err != nil {
		return false, err
	}
	for _, nn := range nodes {
		err = rl.UpsertObjectToItems(nn,
			func(_, _ *fn.KubeObject) bool { return false }, // No de-duplication
			false)
		if err != nil {
			return false, fmt.Errorf("inserting %v/%v: %v", nn.GetKind(), nn.GetName(), err)
		}
	}

	return true, nil
}

func main() {
	if err := fn.AsMain(fn.ResourceListProcessorFunc(Run)); err != nil {
		os.Exit(1)
	}
}
