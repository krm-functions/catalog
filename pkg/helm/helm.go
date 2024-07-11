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
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/GoogleContainerTools/kpt-functions-sdk/go/fn"
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

// PullChart runs 'helm pull' and returns normalized tarball filename and tarball sha256sum
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

	tarball := chartTarballName(chart)
	if _, err := os.Stat(tarball); err != nil {
		options, err := os.ReadDir(dest)
		if err != nil {
			return "", "", err
		}
		if len(options) != 1 {
			return "", "", fmt.Errorf("cannot determine normalized tarball name, found %d files", len(options))
		}
		tarball = options[0].Name()
	}
	chartShaSum := ChartFileSha256(filepath.Join(dest, tarball)) // TODO: Compare with .prov file content
	return tarball, chartShaSum, nil
}

// SourceChart runs PullChart to retrieve chart and reads and returns raw tarball bytes
func SourceChart(chart *t.HelmChart, username, password *string) (chartData []byte, chartSha256Sum string, err error) {
	tmpDir, err := os.MkdirTemp("", "chart-")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(tmpDir)

	tarball, chartSum, err := PullChart(chart.Args, tmpDir, username, password)
	if err != nil {
		return nil, "", err
	}
	buf, err := os.ReadFile(filepath.Join(tmpDir, tarball))
	if err != nil {
		return nil, "", err
	}
	return buf, chartSum, err
}

func isOciRepo(chart t.HelmChartArgs) bool {
	return strings.HasPrefix(chart.Repo, "oci://")
}

// chartTarballName returns the normalized tarball name 'name-v1.2.3.tgz'
func chartTarballName(chart t.HelmChartArgs) string {
	return chart.Name + "-" + chart.Version + ".tgz"
}

func ChartFileSha256(chartFile string) string {
	dat, err := os.ReadFile(chartFile)
	if err != nil {
		panic(fmt.Errorf("cannot read file %q", chartFile))
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

func LookupAuthSecret(secretName, namespace string, rl *fn.ResourceList) (username, password *string, err error) {
	if namespace == "" {
		namespace = "default" // Default according to spec
	}
	for _, k := range rl.Items {
		if !k.IsGVK("v1", "", "Secret") || k.GetName() != secretName {
			continue
		}
		oNamespace := k.GetNamespace()
		if oNamespace == "" {
			oNamespace = "default" // Default according to spec
		}
		if namespace == oNamespace {
			uname, found, err := k.NestedString("data", "username")
			if !found {
				return nil, nil, fmt.Errorf("key 'username' not found in Secret %s/%s", namespace, secretName)
			}
			if err != nil {
				return nil, nil, err
			}
			pword, found, err := k.NestedString("data", "password")
			if !found {
				return nil, nil, fmt.Errorf("key 'password' not found in Secret %s/%s", namespace, secretName)
			}
			if err != nil {
				return nil, nil, err
			}
			u, err := base64.StdEncoding.DecodeString(uname)
			if err != nil {
				return nil, nil, fmt.Errorf("decoding 'username' in Secret %s/%s: %w", namespace, secretName, err)
			}
			uname = string(u)
			p, err := base64.StdEncoding.DecodeString(pword)
			if err != nil {
				return nil, nil, fmt.Errorf("decoding 'password' in Secret %s/%s: %w", namespace, secretName, err)
			}
			pword = string(p)
			return &uname, &pword, nil
		}
	}
	return nil, nil, fmt.Errorf("auth Secret %s/%s not found", namespace, secretName)
}
