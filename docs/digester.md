# Digester

Using digests instead of tags to reference container images is a good
[best
practice](https://medium.com/@michael.vittrup.larsen/why-we-should-use-latest-tag-on-container-images-fc0266877ab5). This
`digester` KRM function implement 'trust on first use' for container
images used in Helm charts. This is done through the following
process:

1. Inspection of Helm chart resources for container images.
2. Lookup of container image digests for container images referenced by tag.
3. **Update chart values with container image digests** by creating values for missing container image digests.

This procedure **keeps the Helm chart configurable and renderable** and
this is **where this function differs from other 'last mile' tools that
operate on the final resources** and thus do not keep Helm charts
renderable.

This `digester` function particularly support a process where
third-party Helm charts are in-sourced into an organisation and
configured with organisation-specific settings, thereby producing
curated Helm-based components:

1. In-source Helm chart using [`source-helm-chart`](source-helm-chart.md).
2. Lookup digests and update Helm image references using this `digester` function. Store the result as a 'curated component'.
3. As needed, post-configure and render Helm chart using [`render-helm-chart`](render-helm-chart.md).

With this process, both the Helm chart and container images are kept
immutable.

The consideration made during the design of `digester` can be found in
the [digester-design document](digester-design.md).

## Example

Imagine the following Helm chart configuration using a
`RenderHelmChart` resource. Notice how the `valuesInline` section at
the end provide simple pre-configuration of the chart beyond its
defaults. A real-world use-case typically will have much more
pre-configuration:

```yaml
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
            team-name: dev  # kpt-set: ${teamName}
```

If we rendered the chart as-is, all container images would be
referenced using tags since we are missing values settings for all
container images. Normally we would have to fill in the following
digest values manually:

```yaml
      ...
      valuesInline:
        global:
          commonLabels:
            team-name: dev  # kpt-set: ${teamName}
        image:
          digest: ""
        webhook:
          image:
            digest: ""
        cainjector:
          image:
            digest: ""
        startupapicheck:
          image:
            digest: ""
```

The `digester` function can automate this process when used after the
[`source-helm-chart`](source-helm-chart.md) function and implements
[option 4 in the design document](digester-design.md). This means that
we only need to manually specify which values settings should be
updated and the `digester` thus operates much like an `apply-setters`
KRM function. However, instead of `apply-setters` variables,
`digester` substitutes container image digests based on a regular
expression matches against tag-based images. The following illustrates
how we specify, through `apply-setter` style comments, the regexp that
should be used to lookup a given digest based on the images the chart
references.

```yaml
      valuesInline:
        global:
          commonLabels:
            team-name: dev  # kpt-set: ${teamName}
        image:
          digest: ""   # digester: quay.io/jetstack/cert-manager-controller:.*
        webhook:
          image:
            digest: "" # digester: quay.io/jetstack/cert-manager-webhook:.*
        cainjector:
          image:
            digest: "" # digester: quay.io/jetstack/cert-manager-cainjector:.*
        startupapicheck:
          image:
            digest: "" # digester: quay.io/jetstack/cert-manager-ctl:.*
```

The process for using `digester` could be:

1. Source Helm chart using [`source-helm-chart`](source-helm-chart.md)
2. Pass the `RenderHelmChart` resource through `digester`, which will:
   - Render Helm chart with given values (`team-name` only in our example).
   - Inspect all rendered resources for fields ending with `containers[].image` or `initContainers[].images`
   - For all container images not already using digests, resolve tags to digests. This implements 'trust on first use'.
   - Re-visit the `RenderHelmChart` resource and update values in `apply-setter` style, using the regular expression given in comments for lookup of digests identified above.
  e. Output of `digester` function is the input resources with `RenderHelmChart` resource(s) updated accordingly. Rendered resources are only used to implement image digest lookup and discarded.

The output of the process above may result in a `RenderHelmChart`
resource that looks like (abbreviated slightly for clarity):

```yaml
apiVersion: experimental.helm.sh/v1alpha1
kind: RenderHelmChart
metadata:
  name: render-chart
  annotations:
    config.kubernetes.io/local-config: "true"
    experimental.helm.sh/chart-sum/cert-manager: sha256:552561ed2dfd3b36553934327034d1dd58ead06b0166eb3eb29c7ad3ca0b8248
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
        image:
          digest: "sha256:5e38e4d06c412e8e3500c857adfe636463aba7261e262b386e12dc4333109a63" # digester: quay.io/jetstack/cert-manager-controller:.*
        webhook:
          image:
            digest: "sha256:78d5d4f21b1daba91ce38918149a9420895daeef15884bb2dccc9ea3178fac78" # digester: quay.io/jetstack/cert-manager-webhook:.*
        cainjector:
          image:
            digest: "sha256:bee98e39e7d5b421c41507665779e816ce8dacf69e9feb3e28b1110391c710c6" # digester: quay.io/jetstack/cert-manager-cainjector:.*
        startupapicheck:
          image:
            digest: "sha256:74023f3ad71915c3d4d249c5a20c7384e377558a030055215e8aeff5112aab4b" # digester: quay.io/jetstack/cert-manager-ctl:.*
  chart: H4sIFAAAAAAA/ykAK2FIUjBjSE02THk5NWIzVjBkUzVpWlM5Nk9WVjZNV2xqYW5keVRRbz1IZWxtAOz9...OY8SOAB6CAA=
```

Full example:

```shell
export SOURCE_HELM_CHART_IMAGE=ghcr.io/krm-functions/source-helm-chart@sha256:1cf52c5bddf70f10ae55f8d5225a55e2e929db6d357c485af5e1ebf816afbcc5
export DIGESTER_IMAGE=ghcr.io/krm-functions/digester@sha256:8c2cac5e8301dfc573661d7cb0f8481bac5d6d2d4d9619a5942ccfb41cc52c44

echo "### Sourcing step - fetches chart and resolves digests, stores immutable package in 'cert-manager-package'"
kpt fn source examples/digester \
  | kpt fn eval - --network --image $SOURCE_HELM_CHART_IMAGE \
  | kpt fn eval - --network --truncate-output=false --image $DIGESTER_IMAGE \
  | kpt fn sink cert-manager-package

echo "### Rendering step - using a declarative pipeline defined in Kptfile which applies gatekeeper policy to check for missing image digests"

kpt fn render cert-manager-package -o stdout | kpt fn sink cert-manager-rendered
```

## Notes

:construction: This function does not yet support private registries.
