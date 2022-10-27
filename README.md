# Helm Chart Upgrader KRM Function `helm-upgrader`

## Overview

The `helm-upgrader` KRM function upgrades Helm chart specs in
[ArgoCD](https://argo-cd.readthedocs.io/en/stable/operator-manual/application.yaml)
and [kpt render-helm-chart
format](https://catalog.kpt.dev/render-helm-chart/v0.2/).

E.g. an ArgoCD Helm chart specification deploying the `cert-manager` Helm chart
may look like:

```
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

The chart version specified here `v1.8.1` is not the most recent version, and
keeping chart version updated is a tedious and on-going activity. This KRM
function automates this process.

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

```
git clone https://github.com/MichaelVL/helm-upgrader.git
cd helm-upgrader

export VERSION=5acbb87

kpt fn source examples | \
  kpt fn eval - --image ghcr.io/michaelvl/helm-upgrader:$VERSION \
    --network --mount type=tmpfs,target=/tmp,rw=true --fn-config examples/config-upgrade-helm-version-inline.yaml | \
  kpt fn sink examples-upgraded
```

The command above will process the manifests in the `examples` folder, run the
`helm-upgrader` KRM function and write-back the Kubernetes manifests into
`examples-upgraded`.

Run `diff` to see the upgraded Helm charts:

```
diff -r examples examples-upgraded
```

The output will contain lines like:

```
diff -r examples/argo-app-cert-manager.yaml examples2/argo-app-cert-manager.yaml
15c16
<     targetRevision: v1.8.1
---
>     targetRevision: v1.8.2
```

which shows that the function upgraded a chart.

## Configuring the Upgrade Process

### Upgrade Constraints

```
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: cert-manager
  annotations:
    experimental.helm.sh/upgrade-constraint: "1.8.*"
```

### Annotate Instead of Upgrade

```
metadata:
  annotations:
    experimental.helm.sh/upgrade-available: https://charts.jetstack.io/cert-manager:v1.8.2
    experimental.helm.sh/upgrade-chart-sum: sha256:b8d0dd5c95398db9308b649f7ef70ca3a0db1bb8859b43f9672c7f66871d0ef9
```
