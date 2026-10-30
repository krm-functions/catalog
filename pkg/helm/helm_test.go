// Copyright 2026 Michael Vittrup Larsen
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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	specs "github.com/krm-functions/catalog/pkg/helmspecs"
	a "github.com/stretchr/testify/assert"
	r "github.com/stretchr/testify/require"
)

const chartYaml = `apiVersion: v2
name: test-chart
description: A Helm test chart
type: application
version: %s
appVersion: "1.16.0"
`

var chartTemplates = map[string]string{
	"templates/configmap.yaml": `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-foo
  namespace: {{ .Release.Namespace }}
data:
  kubeVersion: {{ .Capabilities.KubeVersion.Version | quote }}
  kubeVersionMinor: {{ .Capabilities.KubeVersion.Minor | quote }}
  greeting: {{ .Values.greeting | quote }}
  replicas: {{ .Values.replicas | quote }}
  hasFooAPI: {{ .Capabilities.APIVersions.Has "foo.example.com/v1" | quote }}
`,
	"templates/tests/test.yaml": `apiVersion: v1
kind: Pod
metadata:
  name: {{ .Release.Name }}-test
  annotations:
    "helm.sh/hook": test
`,
	"crds/crd.yaml": `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: foos.example.com
`,
	"values.yaml": "greeting: hello\nreplicas: 1\n",
}

// chartTarball creates a chart tarball in the format created by 'helm package'
func chartTarball(t *testing.T, version string) []byte {
	t.Helper()
	files := map[string]string{"Chart.yaml": fmt.Sprintf(chartYaml, version)}
	for k, v := range chartTemplates {
		files[k] = v
	}
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	for name, content := range files {
		hdr := &tar.Header{Name: filepath.Join("test-chart", name), Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		r.NoError(t, tw.WriteHeader(hdr))
		_, err := tw.Write([]byte(content))
		r.NoError(t, err)
	}
	r.NoError(t, tw.Close())
	r.NoError(t, gzw.Close())
	return buf.Bytes()
}

func TestTemplate(t *testing.T) {
	tarball := chartTarball(t, "0.1.0")

	chart := &specs.HelmChart{Args: specs.HelmChartArgs{Name: "test-chart"}}
	out, err := Template(chart, tarball)
	r.NoError(t, err)
	a.Contains(t, string(out), "name: release-name-foo")
	a.Contains(t, string(out), "namespace: default")
	a.Contains(t, string(out), `greeting: "hello"`)
	a.Contains(t, string(out), `hasFooAPI: "false"`)
	a.Contains(t, string(out), "name: release-name-test", "test hooks rendered by default")
	a.NotContains(t, string(out), "CustomResourceDefinition")

	chart.Options.ReleaseName = "myrel"
	chart.Options.Namespace = "myns"
	chart.Options.KubeVersion = "1.29.0"
	chart.Options.APIVersions = []string{"foo.example.com/v1"}
	chart.Options.IncludeCRDs = true
	chart.Options.SkipTests = true
	chart.Options.Values.ValuesInline = map[string]any{"greeting": "hi", "replicas": 3}
	out, err = Template(chart, tarball)
	r.NoError(t, err)
	a.Contains(t, string(out), "name: myrel-foo")
	a.Contains(t, string(out), "namespace: myns")
	a.Contains(t, string(out), `kubeVersion: "v1.29.0"`)
	a.Contains(t, string(out), `kubeVersionMinor: "29"`)
	a.Contains(t, string(out), `hasFooAPI: "true"`)
	a.Contains(t, string(out), `greeting: "hi"`)
	a.Contains(t, string(out), `replicas: "3"`)
	a.Contains(t, string(out), "CustomResourceDefinition")
	a.NotContains(t, string(out), "myrel-test")

	objs, err := ParseAsKubeObjects(out)
	r.NoError(t, err)
	a.Len(t, objs, 2)

	chart.Options.NameTemplate = "foo"
	_, err = Template(chart, tarball)
	a.Error(t, err, "release name and name template are mutually exclusive")
	chart.Options.ReleaseName = ""
	out, err = Template(chart, tarball)
	r.NoError(t, err)
	a.Contains(t, string(out), "name: foo-foo")

	chart.Options.KubeVersion = "not-a-version"
	_, err = Template(chart, tarball)
	a.Error(t, err)
}

// chartRepoServer serves a Helm repo with a stable and a pre-release chart version
func chartRepoServer(t *testing.T) (srv *httptest.Server, tarball []byte) {
	t.Helper()
	tarball = chartTarball(t, "0.1.0")
	index := `apiVersion: v1
entries:
  test-chart:
  - name: test-chart
    version: 0.1.0
    appVersion: 1.16.0
    description: A Helm test chart
    urls: [test-chart-0.1.0.tgz]
  - name: test-chart
    version: 0.2.0-rc.1
    urls: [test-chart-0.2.0-rc.1.tgz]
  other-chart:
  - name: other-chart
    version: 1.0.0
    urls: [other-chart-1.0.0.tgz]
`
	mux := http.NewServeMux()
	mux.HandleFunc("/index.yaml", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, index) })
	mux.HandleFunc("/test-chart-0.1.0.tgz", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(tarball) })
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, tarball
}

func TestSearchRepo(t *testing.T) {
	srv, _ := chartRepoServer(t)

	search, err := SearchRepo(&specs.HelmChartArgs{Name: "test-chart", Repo: srv.URL}, "", "")
	r.NoError(t, err)
	a.Equal(t, []RepoSearch{{Version: "0.1.0", AppVersion: "1.16.0", Name: "test-chart", Description: "A Helm test chart"}}, search)

	_, err = SearchRepo(&specs.HelmChartArgs{Name: "test-chart", Repo: srv.URL + "/nonexisting"}, "", "")
	a.Error(t, err)
}

func TestPullChart(t *testing.T) {
	srv, tarball := chartRepoServer(t)
	chart := &specs.HelmChartArgs{Name: "test-chart", Version: "0.1.0", Repo: srv.URL}

	dest := t.TempDir()
	name, sum, err := PullChart(chart, dest, "", "")
	r.NoError(t, err)
	a.Equal(t, "test-chart-0.1.0.tgz", name)
	a.Equal(t, fmt.Sprintf("%x", sha256.Sum256(tarball)), sum)
	a.FileExists(t, filepath.Join(dest, name))

	data, _, sum2, err := SourceChart(chart, "", "", "")
	r.NoError(t, err)
	a.Equal(t, tarball, data)
	a.Equal(t, sum, sum2)

	chart.Version = "9.9.9"
	_, _, err = PullChart(chart, t.TempDir(), "", "")
	a.Error(t, err)
}

// TestPullChartNoHome mimics running in a container where HOME is not writable
func TestPullChartNoHome(t *testing.T) {
	srv, _ := chartRepoServer(t)
	chart := &specs.HelmChartArgs{Name: "test-chart", Version: "0.1.0", Repo: srv.URL}

	t.Setenv("HOME", "/nonexistent")
	for _, v := range []string{"HELM_CACHE_HOME", "HELM_CONFIG_HOME", "HELM_DATA_HOME", "XDG_CACHE_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME"} {
		t.Setenv(v, "")
	}
	_, _, err := PullChart(chart, t.TempDir(), "", "")
	r.NoError(t, err)
}
