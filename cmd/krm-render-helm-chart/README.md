# Render Helm Chart Function

TL;DR - this `render-helm-chart` KRM function solves the problem of
the baseline version that it can only be executed imperatively and
thus not be used in declarative KRM function pipelines.

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
  annotations:
    config.kubernetes.io/local-config: "true"
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
```

In this spec, the `chartArgs` part is 'how to source' the Helm chart,
i.e. this points to a network location where the chart can be
located. Because this is a network location, the function can only be
run imperatively with `kpt fn eval --network --image
gcr.io/kpt-fn/render-helm-chart:v0.2.2 ...`

Using a network location in a render operation is troublesome for many
reasons - availability and multiple facets of security being two
important reasons.

## The Solution

The solution is to do away with the need to fetch the Helm chart
through a network. Instead we use the approach we would use if we were
collection plain Kubernetes YAML manifests into a package. **We fetch
the upstream source manifests and store them in our own 'package' Git
repository**. This has many benefits - availability, immutability
etc.

If we similarly retrieve the Helm chart from its upstream source and
store the chart inside a `RenderHelmChart` resource in base64 encoded
form, we can render the chart with a hermetic KRM function through a
declarative pipeline.

The following example illustrates the example from above in this
alternative form. The `templateOptions` part has been retained as this
is specification for how to render the chart. However, the `chartArgs`
part has been replaced by the actual chart in base64 encoded form
as `chart`:

```
apiVersion: experimental.helm.sh/v1alpha1
kind: RenderHelmChart
metadata:
  name: cert-manager
spec:
  templateOptions:
    releaseName: cert-manager
    namespace: cert-manager
    values:
      valuesInline:
        global:
          commonLabels:
            team_name: dev
  chart:            # base64 encoded helm chart file 'cert-manager-v1.9.0.tgz'
    H4sIFAAAAAAA/ykAK2FIUjBjSE02THk5NWIzVjBkUzVpWlM5Nk9WVjZNV2xqYW5keVRRbz1IZWxt
    AOz9+3bbNtY4DM/fugp8zvNbivtJ8il2Ez/TeR6PnXY8TRwv251TZ35jiIQk1CTAAqAdtZN7ea/l
	...
```

The script [`source-chart.sh`](source-chart.sh) implements this
conversion, including fetching the chart.

For completeness and possibly future extensions, the script also adds
the following annotations:

```
apiVersion: experimental.helm.sh/v1alpha1
kind: RenderHelmChart
metadata:
  name: cert-manager
  annotations:
    experimental.helm.sh/chart-sum: "sha256:fab4457eea49344917167f02732fbe56bedbe6ae1935dace8db3fac34d672e85"
    experimental.helm.sh/chart-name: "cert-manager"
    experimental.helm.sh/chart-repo: "https://charts.jetstack.io"
    experimental.helm.sh/chart-version: "v1.9.0"
...
```
