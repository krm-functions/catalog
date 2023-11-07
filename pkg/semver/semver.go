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

package semver

import (
	"fmt"
	"sort"

	version "github.com/Masterminds/semver"
)

func Sort(versionsRaw []string) []*version.Version {
	versions := make([]*version.Version, len(versionsRaw))
	for i, raw := range versionsRaw {
		v, _ := version.NewVersion(raw)
		versions[i] = v
	}

	sort.Sort(sort.Reverse(version.Collection(versions)))
	return versions
}

// func Sort(versions []string) []string {
// 	// Sort, most recent first
// 	semver.Sort(versions)
// 	versions2 := make([]string, len(versions))
// 	for idx, v := range versions {
// 		versions2[len(versions)-idx-1] = v
// 	}
// 	return versions2
// }

func Upgrade(versions []string, constraint string) (string, error) {
	constraints, err := version.NewConstraint(constraint)
	if err != nil {
		return "", fmt.Errorf("error parsing constraint %q: %q", constraint, err.Error())
	}
	vers := Sort(versions)
	for _, v := range vers {
		if constraints.Check(v) {
			return v.Original(), nil
		}
	}
	return "", fmt.Errorf("no version found that satisfies constraint: %q", constraint)
}

func Diff(fromVer, toVer string) (string, error) {
	from, err := version.NewVersion(fromVer)
	if err != nil {
		return "", err
	}
	to, err := version.NewVersion(toVer)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d.%d.%d", to.Major()-from.Major(), to.Minor()-from.Minor(), to.Patch()-from.Patch()), nil
}
