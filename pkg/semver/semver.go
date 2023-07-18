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
