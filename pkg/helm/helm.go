// Copyright 2023 Michael Vittrup Larsen

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at

//     http://www.apache.org/licenses/LICENSE-2.0

// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package helm

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	t "github.com/michaelvl/krm-functions/pkg/helmspecs"
	"github.com/michaelvl/krm-functions/pkg/skopeo"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

type RepoSearch struct {
	Version     string `yaml:"version"`
	AppVersion  string `yaml:"app_version"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type RunContext struct {
	repoConfigDir string
	repoConfig    string
}

func NewRunContext() *RunContext {
	// To avoid modifying local Helm config file, we run Helm using a temporary repo config
	repoCfgDir, err := os.MkdirTemp("", "helm-repo-cfg")
	if err != nil {
		panic(fmt.Errorf("error creating temp Helm config dir: %q", err.Error()))
	}
	return &RunContext{repoCfgDir, filepath.Join(repoCfgDir, "repository")}
}

func (ctxt *RunContext) DiscardContext() {
	os.RemoveAll(ctxt.repoConfigDir)
}

func (ctxt *RunContext) Run(args ...string) ([]byte, error) {
	a := append([]string{"--repository-config", ctxt.repoConfig}, args...)
	cmd := exec.Command("helm", a...)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error running helm command: %q: %q (%q)", args, stderr, err.Error())
	}
	return stdout.Bytes(), nil
}

func SearchRepo(chart t.HelmChartArgs, username, password *string) ([]RepoSearch, error) {
	if isOciRepo(chart) {
		ociSearch, err := skopeo.ListTags(chart)
		if err != nil {
			return nil, err
		}
		versions := make([]RepoSearch, len(ociSearch.Tags))
		for idx, v := range ociSearch.Tags {
			versions[idx].Name = chart.Name
			versions[idx].Version = v
		}
		return versions, nil
	}

	// Plain HTTP Helm repo
	helmCtxt := NewRunContext()
	defer helmCtxt.DiscardContext()

	addArgs := []string{"repo", "add", "tmprepo", chart.Repo}
	if username != nil && password != nil {
		addArgs = append(addArgs, "--username", *username, "--password", *password)
	}
	_, err := helmCtxt.Run(addArgs...)
	if err != nil {
		return nil, err
	}

	_, err = helmCtxt.Run("repo", "update")
	if err != nil {
		return nil, err
	}

	// Search repo for chart, long listing in yaml format. May include other charts partially matching search
	out, err := helmCtxt.Run("search", "repo", "-l", "-o", "yaml", chart.Name)
	if err != nil {
		return nil, err
	}

	var versions []RepoSearch
	if err := kyaml.Unmarshal(out, &versions); err != nil {
		return nil, fmt.Errorf("error parsing helm output: %q", err.Error())
	}

	// Normalize by stripping 'tmprepo/' chart name prefix
	versionsNormalized := make([]RepoSearch, len(versions))
	for idx, v := range versions {
		v.Name = strings.TrimPrefix(v.Name, "tmprepo/")
		versionsNormalized[idx] = v
	}

	return versionsNormalized, nil
}

func PullChart(chart t.HelmChartArgs, destinationPath string, username, password *string) (tarballName, chartSha256Sum string, err error) {
	helmCtxt := NewRunContext()
	defer helmCtxt.DiscardContext()

	var dest string
	if destinationPath == "" {
		dest = helmCtxt.repoConfigDir
	} else {
		dest = destinationPath
	}

	if isOciRepo(chart) {
		_, err := helmCtxt.Run("pull", chart.Repo+"/"+chart.Name, "--version", chart.Version, "--destination", dest)
		if err != nil {
			return "", "", err
		}
	} else {
		repoAlias := "tmprepo"
		addArgs := []string{"repo", "add", repoAlias, chart.Repo}
		if username != nil && password != nil {
			addArgs = append(addArgs, "--username", *username, "--password", *password)
		}
		_, err := helmCtxt.Run(addArgs...)
		if err != nil {
			return "", "", err
		}
		_, err = helmCtxt.Run("repo", "update")
		if err != nil {
			return "", "", err
		}
		_, err = helmCtxt.Run("pull", repoAlias+"/"+chart.Name, "--version", chart.Version, "--destination", dest)
		if err != nil {
			return "", "", err
		}
	}

	chartShaSum := ChartFileSha256(dest, chart) // TODO: Compare with .prov file content
	return chartTarballName(chart), chartShaSum, nil
}

func isOciRepo(chart t.HelmChartArgs) bool {
	return strings.HasPrefix(chart.Repo, "oci://")
}

func chartTarballName(chart t.HelmChartArgs) string {
	return chart.Name + "-" + chart.Version + ".tgz"
}

func ChartFileSha256(pathDir string, chart t.HelmChartArgs) string {
	fn := chartTarballName(chart)
	dat, err := os.ReadFile(filepath.Join(pathDir, fn))
	if err != nil {
		panic(fmt.Errorf("cannot read file %q", fn))
	}
	return fmt.Sprintf("%x", sha256.Sum256(dat))
}

func FilterByChartName(search []RepoSearch, chart t.HelmChartArgs) []RepoSearch {
	var filtered []RepoSearch
	for _, v := range search {
		if v.Name == chart.Name {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

func ToList(search []RepoSearch) []string {
	var versions []string
	for _, v := range search {
		versions = append(versions, v.Version)
	}
	return versions
}
