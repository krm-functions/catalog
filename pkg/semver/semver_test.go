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

package semver

import (
	"testing"
)

func TestUpgrade(t *testing.T) {
	versions := []string{"1.1.0", "1.1.1", "1.1.2", "1.2.0", "1.3.0", "v1.4.0"}

	combs := []struct {
		constraint string
		expect     string
	}{
		{"1.1.*", "1.1.2"},
		{"*", "v1.4.0"},
	}
	for _, test := range combs {
		newVer, err := Upgrade(versions, test.constraint)
		if err != nil {
			t.Errorf("Semver upgrade failure %q", err.Error())
		}
		if newVer != test.expect {
			t.Errorf("Semver upgrade mismatch, got %q from test %+v", newVer, test)
		}
	}
}
