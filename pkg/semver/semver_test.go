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
		new_v, err := Upgrade(versions, test.constraint)
		if err != nil {
			t.Errorf("Semver upgrade failure %q", err.Error())
		}
		if new_v != test.expect {
			t.Errorf("Semver upgrade mismatch, got %q from test %+v", new_v, test)
		}
	}
}
