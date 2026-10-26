// Copyright 2026 Michael Vittrup Larsen
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
package kpt

import (
	"os"
	"path/filepath"
	"testing"

	a "github.com/stretchr/testify/assert"
)

const existingKptfile = `apiVersion: kpt.dev/v1
kind: Kptfile
metadata:
  name: original-name
  namespace: will-be-removed
info:
  description: "" # kpt-set: ${description}
  unknownField: foo
pipeline:
  mutators:
  - image: ghcr.io/krm-functions/set-labels
    configMap:
      app: "" # kpt-set: ${app}
`

const expectedKptfile = `apiVersion: kpt.dev/v1
kind: Kptfile
metadata:
  name: foo
upstream:
  type: git
  git:
    repo: https://github.com/example/repo
    directory: /pkg1
    ref: main
  updateStrategy: resource-merge
upstreamLock:
  type: git
  git:
    repo: https://github.com/example/repo
    directory: /pkg1
    ref: main
    commit: abc123
info:
  description: "" # kpt-set: ${description}
  unknownField: foo
pipeline:
  mutators:
  - image: ghcr.io/krm-functions/set-labels
    configMap:
      app: "" # kpt-set: ${app}
`

const expectedNewKptfile = `apiVersion: kpt.dev/v1
kind: Kptfile
metadata:
  name: foo
upstream:
  type: git
  git:
    repo: https://github.com/example/repo
    directory: /pkg1
    ref: main
  updateStrategy: resource-merge
upstreamLock:
  type: git
  git:
    repo: https://github.com/example/repo
    directory: /pkg1
    ref: main
    commit: abc123
`

func updateMetadata(t *testing.T, dir string) string {
	err := UpdateKptMetadata(dir, "foo", map[string]string{"name": "foo"}, "pkg1", "https://github.com/example/repo", "main", "abc123")
	if err != nil {
		t.Fatalf("UpdateKptMetadata: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "Kptfile"))
	if err != nil {
		t.Fatalf("reading Kptfile: %v", err)
	}
	return string(b)
}

func TestUpdateKptMetadataPreservesFields(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Kptfile"), []byte(existingKptfile), 0o600); err != nil {
		t.Fatal(err)
	}
	a.Equal(t, expectedKptfile, updateMetadata(t, dir))

	// Updating again replaces upstream fields in-place
	a.Equal(t, expectedKptfile, updateMetadata(t, dir))
}

func TestUpdateKptMetadataNewKptfile(t *testing.T) {
	a.Equal(t, expectedNewKptfile, updateMetadata(t, t.TempDir()))
}
