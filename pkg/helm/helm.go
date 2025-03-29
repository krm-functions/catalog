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
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	t "github.com/krm-functions/catalog/pkg/helmspecs"
	"github.com/krm-functions/catalog/pkg/skopeo"
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

func SearchRepo(chart *t.HelmChartArgs, username, password string) ([]RepoSearch, error) {
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
	if username != "" && password != "" {
		addArgs = append(addArgs, "--username", username, "--password", password)
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

// PullChart runs 'helm pull' and returns normalized tarball filename and tarball sha256sum
func PullChart(chart *t.HelmChartArgs, destinationPath, username, password string) (tarballName, chartSha256Sum string, err error) {
	helmCtxt := NewRunContext()
	defer helmCtxt.DiscardContext()

	var dest string
	if destinationPath == "" {
		dest = helmCtxt.repoConfigDir
	} else {
		dest = destinationPath
	}

	if isOciRepo(chart) {
		if username != "" && password != "" {
			loginArgs := []string{"registry", "login", strings.TrimPrefix(chart.Repo, "oci://")}
			loginArgs = append(loginArgs, "--username", username, "--password", password)
			_, err := helmCtxt.Run(loginArgs...)
			if err != nil {
				return "", "", fmt.Errorf("registry login: %w", err)
			}
		}
		_, err := helmCtxt.Run("pull", chart.Repo+"/"+chart.Name, "--version", chart.Version, "--destination", dest)
		if err != nil {
			return "", "", fmt.Errorf("pulling chart (oci): %w", err)
		}
	} else {
		repoAlias := "tmprepo"
		addArgs := []string{"repo", "add", repoAlias, chart.Repo}
		if username != "" && password != "" {
			addArgs = append(addArgs, "--username", username, "--password", password)
		}
		_, err := helmCtxt.Run(addArgs...)
		if err != nil {
			return "", "", fmt.Errorf("adding repo: %w", err)
		}
		_, err = helmCtxt.Run("repo", "update")
		if err != nil {
			return "", "", fmt.Errorf("updating repo: %w", err)
		}
		_, err = helmCtxt.Run("pull", repoAlias+"/"+chart.Name, "--version", chart.Version, "--destination", dest)
		if err != nil {
			return "", "", fmt.Errorf("pulling chart: %w", err)
		}
	}

	tarball := chartTarballName(chart)
	if _, err := os.Stat(tarball); err != nil {
		options, err := os.ReadDir(dest)
		if err != nil {
			return "", "", fmt.Errorf("read dir with chart tarball: %w", err)
		}
		if len(options) != 1 {
			return "", "", fmt.Errorf("cannot determine normalized tarball name, found %d files", len(options))
		}
		tarball = options[0].Name()
	}
	chartShaSum := ChartFileSha256(filepath.Join(dest, tarball)) // TODO: Compare with .prov file content

	return tarball, chartShaSum, nil
}

// SourceChart runs PullChart to retrieve chart and reads and returns raw tarball bytes.
// If destination is not defined, a temporary directory will be used, and cleaned-up at function
// exit. This means that only the returned chartData can be used and not the tarballName
func SourceChart(chart *t.HelmChartArgs, destination, username, password string) (chartData []byte, tarballName, chartSha256Sum string, err error) {
	var tmpDir string
	if destination == "" {
		tmpDir, err = os.MkdirTemp("", "chart-")
		if err != nil {
			return nil, "", "", fmt.Errorf("creating tmp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)
	} else {
		tmpDir = destination
	}

	tarball, chartSum, err := PullChart(chart, tmpDir, username, password)
	if err != nil {
		return nil, "", "", err
	}
	buf, err := os.ReadFile(filepath.Join(tmpDir, tarball))
	if err != nil {
		return nil, "", "", fmt.Errorf("reading chart bytes: %w", err)
	}
	if destination == "" {
		// tarball is deleted, i.e. return empty string
		return buf, "", chartSum, err
	}
	return buf, tarball, chartSum, err
}

func isOciRepo(chart *t.HelmChartArgs) bool {
	return strings.HasPrefix(chart.Repo, "oci://")
}

// chartTarballName returns the normalized tarball name 'name-v1.2.3.tgz'
func chartTarballName(chart *t.HelmChartArgs) string {
	return chart.Name + "-" + chart.Version + ".tgz"
}

func ChartFileSha256(chartFile string) string {
	dat, err := os.ReadFile(chartFile)
	if err != nil {
		panic(fmt.Errorf("cannot read file %q", chartFile))
	}
	return fmt.Sprintf("%x", sha256.Sum256(dat))
}

func FilterByChartName(search []RepoSearch, chart *t.HelmChartArgs) []RepoSearch {
	var filtered []RepoSearch
	for _, v := range search {
		if v.Name == chart.Name {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

// ToList process a repo-search and return a slice with versions
func ToList(search []RepoSearch) []string {
	var versions []string
	for _, s := range search {
		versions = append(versions, s.Version)
	}
	return versions
}

func lookupVersion(search []RepoSearch, version string) *RepoSearch {
	for _, s := range search {
		if s.Version == version {
			return &s
		}
	}
	return nil
}

// GetVersion looks up a specific chart version in repo-search
func GetSearch(search []RepoSearch, version string) (*RepoSearch, error) {
	s := lookupVersion(search, version)
	if s != nil {
		return s, nil
	}
	// Try some Helm isms for better error reporting
	var alt string
	if strings.HasPrefix(version, "v") {
		alt = strings.TrimPrefix(version, "v")
	} else {
		alt = "v" + version
	}
	s = lookupVersion(search, alt)
	if s != nil {
		return nil, fmt.Errorf("version %v not found, did you mean %v", version, alt)
	}
	return nil, fmt.Errorf("version %v not found", version)
}
