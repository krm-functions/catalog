package helm

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	t "github.com/michaelvl/helm-upgrader/pkg/helmspecs"
	"github.com/michaelvl/helm-upgrader/pkg/skopeo"
	"os"
	"os/exec"
	"path/filepath"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
)

type HelmRepoSearch struct {
	Version     string `yaml:"version"`
	AppVersion  string `yaml:"app_version"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type HelmRunContext struct {
	repoConfigDir string
	repoConfig    string
}

func NewRunContext() *HelmRunContext {
	// To avoid modifying local Helm config file, we run Helm using a temporary repo config
	repo_cfg_dir, err := os.MkdirTemp("", "helm-repo-cfg")
	if err != nil {
		panic(fmt.Errorf("Error creating temp Helm config dir: %q", err.Error()))
	}
	return &HelmRunContext{repo_cfg_dir, filepath.Join(repo_cfg_dir, "repository")}
}

func (ctxt *HelmRunContext) DiscardContext() {
	os.RemoveAll(ctxt.repoConfigDir)
}

func (ctxt *HelmRunContext) Run(args ...string) ([]byte, error) {
	a := append([]string{"--repository-config", ctxt.repoConfig}, args...)
	cmd := exec.Command("helm", a...)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("Error running helm command: %q: %q (%q)", args, stderr, err.Error())
	}
	return stdout.Bytes(), nil
}

func RepoSearch(chart t.HelmChartArgs, username, password *string) ([]HelmRepoSearch, error) {
	if isOciRepo(chart) {
		ociSearch, err := skopeo.ListTags(chart)
		if err != nil {
			return nil, err
		}
		versions := make([]HelmRepoSearch, len(ociSearch.Tags))
		for idx, v := range ociSearch.Tags {
			versions[idx].Name = chart.Name
			versions[idx].Version = v
		}
		return versions, nil
	} else {
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

		var versions []HelmRepoSearch
		if err := kyaml.Unmarshal(out, &versions); err != nil {
			return nil, fmt.Errorf("Error parsing helm output: %q", err.Error())
		}

		// Normalize by stripping 'tmprepo/' chart name prefix
		versions_normalized := make([]HelmRepoSearch, len(versions))
		for idx, v := range versions {
			v.Name = strings.TrimPrefix(v.Name, "tmprepo/")
			versions_normalized[idx] = v
		}

		return versions_normalized, nil
	}
}

func PullChart(chart t.HelmChartArgs, destinationPath string, username, password *string) (string, string, error) {
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
		panic(fmt.Errorf("Cannot read file %q", fn))
	}
	return fmt.Sprintf("%x", sha256.Sum256(dat))
}

func FilterByChartName(search []HelmRepoSearch, chart t.HelmChartArgs) []HelmRepoSearch {
	var filtered []HelmRepoSearch
	for _, v := range search {
		if v.Name == chart.Name {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

func ToList(search []HelmRepoSearch) []string {
	var versions []string
	for _, v := range search {
		versions = append(versions, v.Version)
	}
	return versions
}
