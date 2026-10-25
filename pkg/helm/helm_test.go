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
	"testing"

	a "github.com/stretchr/testify/assert"
)

func TestRegistryHost(t *testing.T) {
	tests := []struct {
		repo string
		host string
	}{
		{"oci://ghcr.io/org/charts", "ghcr.io"},
		{"oci://ghcr.io/org/charts/", "ghcr.io"},
		{"oci://localhost:5000/charts", "localhost:5000"},
		{"oci://ghcr.io", "ghcr.io"},
	}
	for _, tc := range tests {
		host, err := registryHost(tc.repo)
		a.NoError(t, err, tc.repo)
		a.Equal(t, tc.host, host, tc.repo)
	}

	for _, repo := range []string{"oci://", "ghcr.io/org/charts", "://bad"} {
		_, err := registryHost(repo)
		a.Error(t, err, repo)
	}
}
