# Helm Chart Upgrader KRM Function `helm-upgrader`

## Overview

The `helm-upgrader` KRM function upgrades Helm chart specs in
[ArgoCD](https://argo-cd.readthedocs.io/en/stable/operator-manual/application.yaml)
and [kpt render-helm-chart
format](https://catalog.kpt.dev/render-helm-chart/v0.2/).

E.g. an ArgoCD Helm chart specification deploying the `cert-manager` Helm chart
may look like:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: cert-manager
spec:
  source:
    chart: cert-manager
    repoURL: https://charts.jetstack.io
    targetRevision: v1.8.1
```

Similarly, the _kpt render-helm-chart_ format may look like this:

```yaml
apiVersion: fn.kpt.dev/v1alpha1
kind: RenderHelmChart
metadata:
  name: cert-manager
helmCharts:
  - chartArgs:
      name: cert-manager
      version: v1.8.1
      repo: https://charts.jetstack.io
    templateOptions:
      releaseName: cert-manager
```

The chart version specified here `v1.8.1` is not the most recent
version, and keeping chart version updated is a tedious and on-going
activity. **This KRM function automates this process.** The following
modes of operation is supported:

- Rewrite the spec with the upgraded chart version according to constraints (see below).
- Annotate the spec when new version is available. This can be useful for manual review and notification procedures.
- Annotate spec with current and new SHA checksum. This is useful for keeping a software delivery chain secure.

## Usage

The `helm-upgrader` function can upgrade the chart version and/or it can provide
information on available upgrades. The latter is convenient if a fully automated
upgrade is not desired. Upgrades can be controlled using constraints on the
sematic versioning, e.g. `1.8.*` allows automated patch version upgrades
only. This mechanism is well-known from many other package managers.

In the following, we will be using [kpt](https://kpt.dev/) for running the KRM
`helm-upgrader` function. See [Replacing Helm and Kustomize with KRM Functions —
a New Approach to Configuration
Management](https://medium.com/@michael.vittrup.larsen/replacing-helm-and-kustomize-with-krm-functions-a-new-approach-to-configuration-management-676212cc1332)
for an introduction to `kpt` and KRM functions.

TL;DR:

Start by cloning this repository with example files:

```shell
git clone https://github.com/krm-functions/catalog.git
cd krm-functions
```

Run the `helm-upgrader` function using `kpt`:

```shell
kpt fn source examples/helm-upgrader | \
  kpt fn eval - \
    --image ghcr.io/krm-functions/helm-upgrader \
    --network --truncate-output=false \
    --fn-config example-function-configs/config-upgrade-helm-version-inline.yaml | \
  kpt fn sink examples-upgraded
```

The command above will process the manifests in the `examples/helm-upgrader` folder, run the
`helm-upgrader` KRM function and write-back the manifests into
`examples-upgraded`.

Run `diff` to see the upgraded Helm charts:

```shell
diff -r examples/helm-upgrader examples-upgraded
```

The output will contain lines like:

```shell
diff -r examples/helm-upgrader/argo-app-cert-manager.yaml examples-upgraded/argo-app-cert-manager.yaml
15c16
<     targetRevision: v1.8.1
---
>     targetRevision: v1.8.2
```

which shows that the function upgraded a chart.

## Configuring the Upgrade Process

### Upgrade Constraints

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: cert-manager
  annotations:
    experimental.helm.sh/upgrade-constraint: "1.8.*"
```

See also [supported upgrade constraints format](https://github.com/Masterminds/semver).

### Annotate Instead of Upgrade

```yaml
metadata:
  annotations:
    experimental.helm.sh/upgrade-available: https://charts.jetstack.io/cert-manager:v1.8.2
    experimental.helm.sh/upgrade-chart-sum: sha256:b8d0dd5c95398db9308b649f7ef70ca3a0db1bb8859b43f9672c7f66871d0ef9
```

## OCI Container Registries

Charts stored in OCI container registries are supported. The chart repository
must start with `oci://` to differentiate from standard HTTP-based chart
repositories. See the example [`examples/krm-metacontroller.yaml`](examples/krm-metacontroller.yaml).

## SemVer Ordering and Difference

Upgrading [semantic versions](https://semver.org/) require that we can
reliably order the versions. If a mixed versioning scheme is used,
e.g. a mix of semver and date-based versions (e.g. '2023-11-11'), then
ordering versions without heuristics is impossible. To handle this the function
only accept semver v2.0.0 versions with the only exception being a
leading 'v'.

## Function Result

This function returns a JSON result, which may look like:

```json
{
  "current": {
    "name": "cert-manager",
    "version": "v1.9.0",
    "repo": "https://charts.jetstack.io",
    "appVersion": "v1.9.0"
  },
  "upgraded": {
    "name": "cert-manager",
    "version": "v1.15.1",
    "repo": "https://charts.jetstack.io"
    "appVersion": "v1.15.1",
	"chartSum":"sha256:b0c179e643bf90d6f74d1c4a8c3e8bede1944ef7828b71419f6384c0157cf8cb"
  },
  "constraint": "",
  "semverDistance": "0.6.0"
}
```

The `semverDistance` is an approximate difference between the current
and most recent version that fulfill the upgrade constrant. Since
semver differences are not well-defined mathematically, the difference
is given by the difference in the left-most place where a difference
is found, hence the 'minor' version in this example.

## Dependencies

This function use [helm](https://helm.sh/) and
[skopeo](https://github.com/containers/skopeo) to retrieve available
chart versions.
