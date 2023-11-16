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

	version "github.com/Masterminds/semver/v3"
)

// Sort versions in descending order
//
// Sorting require, that we can reliably order the versions. If a
// mixed versioning scheme is used, e.g. a mix of semver and
// date-based versions (e.g. '2023-11-11'), then ordering versions
// without heuristics is impossible. To handle this we only accept
// semver v2.0.0 versions with the only exception being a leading 'v'.
func Sort(versionsRaw []string) []*version.Version {
	versions := make([]*version.Version, 0, len(versionsRaw))
	for _, raw := range versionsRaw {
		rawOrig := raw
		if len(raw) > 0 && raw[0] == 'v' {
			raw = raw[1:]
		}
		_, err := version.StrictNewVersion(raw) // Only accept semver-2
		if err == nil {
			v, _ := version.NewVersion(rawOrig) // Parse with optional leading 'v'
			versions = append(versions, v)
		}
	}

	sort.Sort(sort.Reverse(version.Collection(versions)))
	return versions
}

// Upgrade returns the highest version from versions that fulfill constraint
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

// Diff will calculate the difference between two semver
// versions. Since semver are not a well-defined numeric, the
// subtraction is limited to the difference between leftmost non-zero
// difference, i.e. if a difference is found in the major numbers,
// then that difference is returned and the others are represeneted as
// zeros.  E.g. the difference between `2.0.0' and '1.6.99' is
// '1.0.0'
func Diff(fromVer, toVer string) (string, error) {
	from, err := version.NewVersion(fromVer)
	if err != nil {
		return "", err
	}
	to, err := version.NewVersion(toVer)
	if err != nil {
		return "", err
	}
	var major, minor, patch uint64
	major = to.Major() - from.Major()
	if major == 0 {
		minor = to.Minor() - from.Minor()
		if minor == 0 {
			patch = to.Patch() - from.Patch()
		}
	}
	return fmt.Sprintf("%d.%d.%d", major, minor, patch), nil
}
