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
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/crane"
	t "github.com/krm-functions/catalog/pkg/helmspecs"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/helmpath"
	"helm.sh/helm/v4/pkg/registry"
	repo "helm.sh/helm/v4/pkg/repo/v1"
)

type RepoSearch struct {
	Version     string `yaml:"version"`
	AppVersion  string `yaml:"app_version"`
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// newSettings returns Helm settings with repo config and caches in
// dir. This avoids modifying the local Helm config. The Helm home
// env vars are also set, since some Helm code paths ignore settings
// and use helmpath defaults, e.g. repo.FindChartInRepoURL, which
// would otherwise fall back to $HOME/.cache (not writable in containers)
func newSettings(dir string) *cli.EnvSettings {
	os.Setenv(helmpath.CacheHomeEnvVar, filepath.Join(dir, "cache"))
	os.Setenv(helmpath.ConfigHomeEnvVar, filepath.Join(dir, "config"))
	os.Setenv(helmpath.DataHomeEnvVar, filepath.Join(dir, "data"))
	settings := cli.New()
	settings.RepositoryConfig = filepath.Join(dir, "repositories.yaml")
	settings.RepositoryCache = filepath.Join(dir, "repository")
	settings.ContentCache = filepath.Join(dir, "content")
	settings.RegistryConfig = filepath.Join(dir, "registry.json")
	return settings
}

func SearchRepo(chart *t.HelmChartArgs, username, password string) ([]RepoSearch, error) {
	if isOciRepo(chart) {
		return searchOciRepo(chart, username, password)
	}

	// Plain HTTP Helm repo
	tmpDir, err := os.MkdirTemp("", "helm-")
	if err != nil {
		return nil, fmt.Errorf("creating tmp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	settings := newSettings(tmpDir)

	entry := &repo.Entry{Name: "tmprepo", URL: chart.Repo, Username: username, Password: password}
	chartRepo, err := repo.NewChartRepository(entry, getter.All(settings))
	if err != nil {
		return nil, err
	}
	chartRepo.CachePath = settings.RepositoryCache
	indexFile, err := chartRepo.DownloadIndexFile()
	if err != nil {
		return nil, fmt.Errorf("downloading repo index: %w", err)
	}
	index, err := repo.LoadIndexFile(indexFile)
	if err != nil {
		return nil, fmt.Errorf("loading repo index: %w", err)
	}
	index.SortEntries()

	// Only stable releases, same as 'helm search repo' without '--devel'
	stable, err := semver.NewConstraint(">0.0.0")
	if err != nil {
		return nil, err
	}
	var versions []RepoSearch
	for _, cv := range index.Entries[chart.Name] {
		v, err := semver.NewVersion(cv.Version)
		if err != nil || !stable.Check(v) {
			continue
		}
		versions = append(versions, RepoSearch{
			Version:     cv.Version,
			AppVersion:  cv.AppVersion,
			Name:        cv.Name,
			Description: cv.Description,
		})
	}
	return versions, nil
}

// searchOciRepo lists chart versions from the tags of an OCI repo
func searchOciRepo(chart *t.HelmChartArgs, username, password string) ([]RepoSearch, error) {
	ref := strings.TrimPrefix(chart.Repo, "oci://") + "/" + chart.Name
	opt := crane.WithAuthFromKeychain(authn.DefaultKeychain)
	if username != "" && password != "" {
		opt = crane.WithAuth(&authn.Basic{Username: username, Password: password})
	}
	tags, err := crane.ListTags(ref, opt)
	if err != nil {
		return nil, fmt.Errorf("listing tags of %q: %w", ref, err)
	}
	versions := make([]RepoSearch, len(tags))
	for idx, v := range tags {
		versions[idx].Name = chart.Name
		versions[idx].Version = v
	}
	return versions, nil
}

// PullChart pulls a chart like 'helm pull' and returns normalized tarball filename and tarball sha256sum
func PullChart(chart *t.HelmChartArgs, destinationPath, username, password string) (tarballName, chartSha256Sum string, err error) {
	tmpDir, err := os.MkdirTemp("", "helm-")
	if err != nil {
		return "", "", fmt.Errorf("creating tmp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)
	settings := newSettings(tmpDir)

	var dest string
	if destinationPath == "" {
		dest = tmpDir
	} else {
		dest = destinationPath
	}

	cfg := action.NewConfiguration()
	pull := action.NewPull(action.WithConfig(cfg))
	pull.Settings = settings
	pull.Version = chart.Version
	pull.DestDir = dest
	pull.Username = username
	pull.Password = password

	if isOciRepo(chart) {
		opts := []registry.ClientOption{registry.ClientOptCredentialsFile(settings.RegistryConfig)}
		if username != "" && password != "" {
			opts = append(opts, registry.ClientOptBasicAuth(username, password))
		}
		cfg.RegistryClient, err = registry.NewClient(opts...)
		if err != nil {
			return "", "", fmt.Errorf("creating registry client: %w", err)
		}
		if _, err = pull.Run(chart.Repo + "/" + chart.Name); err != nil {
			return "", "", fmt.Errorf("pulling chart (oci): %w", err)
		}
	} else {
		pull.RepoURL = chart.Repo
		if _, err = pull.Run(chart.Name); err != nil {
			return "", "", fmt.Errorf("pulling chart: %w", err)
		}
	}

	tarball := chartTarballName(chart)
	if _, err := os.Stat(filepath.Join(dest, tarball)); err != nil {
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
