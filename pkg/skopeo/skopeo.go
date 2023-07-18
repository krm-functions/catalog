package skopeo

import (
	"bytes"
	"fmt"
	"os/exec"
	"regexp"

	t "github.com/michaelvl/helm-upgrader/pkg/helmspecs"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
)

type RepoTags struct {
	Repository string   `yaml:"Repository"`
	Tags       []string `yaml:"Tags"`
}

func Run(args ...string) ([]byte, error) {
	cmd := exec.Command("skopeo", args...)
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error running skopeo command: %q: %q", args, err.Error())
	}
	return stdout.Bytes(), nil
}

func ListTags(chart t.HelmChartArgs) (*RepoTags, error) {
	repo := regexp.MustCompile("^oci://").ReplaceAllString(chart.Repo, "docker://")
	out, err := Run("list-tags", repo+"/"+chart.Name)
	if err != nil {
		return nil, err
	}

	var search RepoTags
	if err := kyaml.Unmarshal(out, &search); err != nil {
		return nil, fmt.Errorf("error parsing skopeo output: %q", err.Error())
	}
	return &search, nil
}
