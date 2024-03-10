# Render Helm Chart Function

TL;DR - this `render-helm-chart` KRM function supports Helm chart
rendering in declarative pipelines and thus solves the problem of the
[baseline version](https://catalog.kpt.dev/render-helm-chart/v0.2/)
that it can only be executed imperatively.

## The Problem of the Baseline `render-helm-chart`

The [baseline
version](https://catalog.kpt.dev/render-helm-chart/v0.2/) of the
`render-helm-chart` KRM function accepts a `RenderHelmChart` spec as
shown in the following example:

```
apiVersion: fn.kpt.dev/v1alpha1
kind: RenderHelmChart
metadata:
  name: render-chart
helmCharts:
- chartArgs:                   # --- How to source the chart
    name: cert-manager
    version: v1.9.0
    repo: https://charts.jetstack.io
  templateOptions:             # --- How to render the chart
    releaseName: cert-manager
    namespace: cert-manager
    values:
      valuesInline:
        global:
          commonLabels:
            team_name: dev
```

In this spec, the `chartArgs` section define how to source the Helm
chart, i.e. this points to a network location where the chart can be
retrieved from. Because this is a network location, the function can
only be run imperatively with `kpt fn eval --network --image
gcr.io/kpt-fn/render-helm-chart:v0.2.2 ...`

Using a network location in a render operation is troublesome for many
reasons - availability and multiple facets of security being two
important reasons.

## The Solution

A solution is to **separate sourcing and rendering of the Helm
chart**.  This approach is similar to how we would handle a collection
of plain Kubernetes YAML manifests in a package:

1. Fetch YAML manifests from upstream source.
2. Store the upstream manifests in our own Git repository, optionally with
   additional YAML manifests. This is our *curated* package.
3. Consume our package with our Git repository as single source.
4. Modify our package as needed with KRM function pipelines.

**Hence, we fetch the upstream source manifests and store them in our
own 'package' Git repository**. This has many benefits - availability,
immutability etc.

If we similarly retrieve the Helm chart tar-ball from its upstream
source and store the full chart inside a `RenderHelmChart` resource in
base64 encoded form, **we can render the chart with a hermetic KRM
function through a declarative pipeline**. Although the chart is not
plain YAML, the principle is the same as for ordinary YAML manifests.

The following example illustrates the example from above in this
alternative form - note how a `chart` section have been added:

```
apiVersion: experimental.helm.sh/v1alpha1
kind: RenderHelmChart
metadata:
  name: cert-manager
helmCharts:
- chartArgs:
    name: cert-manager
    version: v1.9.0
    repo: https://charts.jetstack.io
  templateOptions:
    releaseName: cert-manager
    namespace: cert-manager
    values:
      valuesInline:
        global:
          commonLabels:
            team_name: dev
  chart: |
    H4sIFAAAAAAA/ykAK2FIUjBjSE02THk5NWIzVjBkUzVpWlM5Nk9WVjZNV2xqYW5keVRRbz1IZWxt
    AOz9+3bbNtY4DM/fugp8zvNbivtJ8il2Ez/TeR6PnXY8TRwv251TZ35jiIQk1CTAAqAdtZN7ea/l
    vbJ3YeNAkKIkSlZSpyW7VmOROG4A+4y9IyJUP8UMj4nYOZ1goQZTnCa/2+Szu7u7e/TiBfy7u7tb
  ...
```

Note, the `chartArgs` map may seem unnecessary after sourcing the Helm
chart, however, keeping it in the `RenderHelmChart` specification
allows for chart updates through modification of the `version` field
and re-sourcing of the Helm chart - possibly using the [helm-upgrader
function](docs/helm-upgrader.md).

The script [`source-chart.sh`](source-chart.sh) implements this
conversion, including fetching the chart. The `render-helm-chart`
function is also dual-purpose, i.e. it can both source and render Helm
charts. See the example below for an example.

For completeness and possibly future extensions, the script also adds
the following annotation:

```
apiVersion: experimental.helm.sh/v1alpha1
kind: RenderHelmChart
metadata:
  name: cert-manager
  annotations:
    experimental.helm.sh/chart-sum: "sha256:fab4457eea49344917167f02732fbe56bedbe6ae1935dace8db3fac34d672e85"
...
```

## FunctionConfig or ResourceList as Input?

This function reads the `RenderHelmChart` resource from the items in
the input `ResourceList` and does not pass the `RenderHelmChart`
resource to the output. This is different that the [upstream
`render-helm-chart`](https://catalog.kpt.dev/render-helm-chart/v0.2/)
which reads the `RenderHelmChart` resource from `FunctionConfig`.

## Example Usage

The file `examples/render-helm-chart/cert-manager-chart.yaml` have an
example `RenderHelmChart` specification with `apiVersion:
fn.kpt.dev/v1alpha1`:

```
# examples/render-helm-chart/cert-manager-chart.yaml
apiVersion: fn.kpt.dev/v1alpha1
kind: RenderHelmChart
metadata:
  name: render-chart
  annotations:
    config.kubernetes.io/local-config: "true"
helmCharts:
- chartArgs:
    name: cert-manager
    version: v1.12.2
    repo: https://charts.jetstack.io
  templateOptions:
    releaseName: cert-managerrel
    namespace: cert-managerns
    values:
      valuesInline:
        global:
          commonLabels:
            team_name: dev  # kpt-set: ${teamName}
```

First we source the Helm chart using the `render-helm-chart` function using an imperative execution:

```
export RENDER_HELM_CHART_IMAGE=ghcr.io/michaelvl/krm-render-helm-chart@sha256:07151b9a8a7cc0cf435ee794eb72009586f54b5113c1b92b20c090b11528f23c

kpt fn source examples/render-helm-chart \
 | kpt fn eval - --network -i $RENDER_HELM_CHART_IMAGE \
 | kpt fn sink my-cert-manager-package
```

Now `my-cert-manager-package/cert-manager-chart.yaml` holds your Helm
chart package with the Helm chart embedded. The apiVersion have been
changed to `apiVersion: experimental.helm.sh/v1alpha1`:

```
# my-cert-manager-package/cert-manager-chart.yaml
apiVersion: experimental.helm.sh/v1alpha1
kind: RenderHelmChart
metadata:
  name: render-chart
  annotations:
    config.kubernetes.io/local-config: "true"
    experimental.helm.sh/chart-sum: sha256:552561ed2dfd3b36553934327034d1dd58ead06b0166eb3eb29c7ad3ca0b8248
helmCharts:
- chartArgs:
    name: cert-manager
    version: v1.12.2
    repo: https://charts.jetstack.io
  templateOptions:
    releaseName: cert-managerrel
    namespace: cert-managerns
    values:
      valuesInline:
        global:
          commonLabels:
            team_name: dev # kpt-set: ${teamName}
  chart: H4sIFAAAAAAA/ykAK2FIUjBjSE02THk5NWIzVjBkUzVpWlM5Nk9WVjZ ... <shorted for clarity!>
```

The chart have not been rendered, only sourced.

Next, we will render the chart using a declarative pipeline. Note,
that the chart values have an `kpt-set`.

We create a pipeline using a `Kptfile` that use the
`render-helm-chart` KRM function and an `apply-setters` to change the
label in the values of the chart:

```
# examples/render-helm-chart/Kptfile
apiVersion: kpt.dev/v1
kind: Kptfile
metadata:
  name: subpkg
  annotations:
    config.kubernetes.io/local-config: "true"
info:
  description: sample description
pipeline:
  mutators:
  - image: gcr.io/kpt-fn/apply-setters:v0.2.0
    configMap:
      teamName: blue-team
  - image: ghcr.io/michaelvl/krm-render-helm-chart@sha256:07151b9a8a7cc0cf435ee794eb72009586f54b5113c1b92b20c090b11528f23c
  - image: gcr.io/kpt-fn/remove-local-config-resources:v0.1.0
```

Copy the `Kptfile` to the package folder:

```
cp examples/render-helm-chart/Kptfile my-cert-manager-package/
```

Next, render the chart using the declarative pipeline in the
`Kptfile`:

```
kpt fn render my-cert-manager-package -o stdout
```

Notice how the labels on the rendered chart have `team_name: blue-team`.
