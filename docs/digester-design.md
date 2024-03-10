# Digester Design Document

Using digests instead of tags for container images is a good [best practice](https://medium.com/@michael.vittrup.larsen/why-we-should-use-latest-tag-on-container-images-fc0266877ab5).

However, with third-party Helm charts this is often cumbersome. Often we see Helm chart defaults like:

```yaml
# values.yaml (chart default values)
image:
  repository: foobar
  tag: v1.2.3
```

These default values are then used in Helm templates like:

```yaml
# templates/deployment.yaml
apiVersion: apps/v1
kind: Deployment
...
spec:
  ...
    containers:
    - image: "{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}"
```

It is possible to post-process a rendered Helm chart using
e.g. [k8s-digester](https://github.com/google/k8s-digester) and
several tools provide Kubernetes mutating webhooks that resolves tags
into digests (e.g. Knative and Sigstore policy-controller). However,
the problem with such a *trust on first use* in the context of a
cluster is that it is a rather late process and that the digest is not
immutable. Imagine deploying a container to two different cluster as
different times and the tag having been moved between the first and
second deployment time.

## Freezing Tags/Digests and Keep Helm Charts Configurable

Imagine the example below - this is a specification of a Helm chart
with some curated values (last lines, abbreviated for clarity). Note
particularly how the common label `team_name` is kept
configurable. I.e. it would be possible to pass this resource through
a KRM [`apply-setters`](https://catalog.kpt.dev/apply-setters/v0.2/)
function before rendering the chart. Hence we can do **high-level
configuration** on the Helm chart values.

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
            team_name: dev  # kpt-set: ${teamName}

        # more curated settings here...
        resources:
          requests:
            cpu: 1
            # ...
```

The `values.yaml` for the example chart shown above also have:

```
# values.yaml
image:
  repository: quay.io/jetstack/cert-manager-controller
  # You can manage a registry with
  # registry: quay.io
  # repository: jetstack/cert-manager-controller

  # Override the image tag to deploy by setting this variable.
  # If no value is set, the chart's appVersion will be used.
  # tag: canary

  # Setting a digest will override any tag
  # digest: sha256:0e072dddd1f7f8fc8909a2ca6f65e76c5f0d2fcfb8be47935ae3457e8bbceb20
```

and a template of the chart refers to container image with:

```yaml
# deployment.yaml
...
image: "{{- if .registry -}}{{ .registry }}/{{- end -}}{{ .repository }}{{- if (.digest) -}} @{{ .digest }}{{- else -}}:{{ default $.Chart.AppVersion .tag }} {{- end -}}"
```

The
[`render-helm-chart`](https://github.com/michaelvl/krm-functions/blob/main/docs/render-helm-chart.md)
function provides declarative Helm chart rendering from a
[`Kptfile`](https://kpt.dev/book/04-using-functions/01-declarative-function-execution). This
is made possible through separation of Helm chart sourcing and chart
rendering. Ideally, **the source stage of `render-hewlm-chart` should
resolve image tags into digests and write the digest into Helm chart
values**.

E.g., when sourcing the chart above, the `RenderHelmChart` resource
should be augmented with a digest setting:

```yaml
apiVersion: experimental.helm.sh/v1alpha1
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
        image:
          digest: abc123def456    ### <---- resolved as part of chart sourcing
  chart: |
    H4sIFAAAAAAA/ykAK2FIUjBjSE02THk5NWIzVjBkUzVpWlM5Nk9WVjZNV2xqYW5keVRRbz1IZWxt
    AOz9+3bbNtY4DM/fugp8zvNbivtJ8il2Ez/TeR6PnXY8TRwv251TZ35jiIQk1CTAAqAdtZN7ea/l
    vbJ3YeNAkKIkSlZSpyW7VmOROG4A+4y9IyJUP8UMj4nYOZ1goQZTnCa/2+Szu7u7e/TiBfy7u7tb
    ...
```

**This makes both the Helm chart and the associated container images
immutable while keeping the Helm chart configurable.**

## All Charts are Different

There is no common standard for how Helm charts structure their
values. Hence we cannot e.g. rely on the digest being stored in
`image.digest`. Image URIs may be hardcoded in templates or consist of
several value elements, e.g. both `registry` and `repo` as in the
example above.

Simultaneously we want to work on the 'Helm chart values' abstraction
level, i.e. we do not want to render the chart and then post-process
it. We want to keep the chart in its un-rendered form such that we at
a later point can render it with a partially changed configuration,
e.g. changing the `resources.request.cpu` in the example above.

The following sections describe options for implementing image digest
resolution.

### Option 1 - 

Copy the image URI template from Helm templates and define the path
where the resolved digest should be stored in the values:

```yaml
imageTemplate: "{{- if .Values.image.registry -}}{{ .Values.image.registry }}/{{- end -}}{{ .Values.image.repository }}{{- if (.Values.image.digest) -}} @{{ .Values.image.digest }}{{- else -}}:{{ default .Chart.AppVersion .tag }} {{- end -}}"
digestPath: image.digest
```

With a definition as above the URI in `imageTempalte` is rendered
using chart values, the digest resolved and added to the chart values
at location `image.digest`.

### Option 2 - 

Render templates with given values, search for image URIs in
well-known resources types (similar to what `k8s-digester` does) and
define the path where the resolved digest should be stored in the
values:

```yaml
imageRegexp: "quay.io/jetstack/cert-manager-controller:.*"
digestPath: image.digest
```

## Evaluation

- Option 1 is dependent on the chart templates because the image URI templates is copied from the chart templates. Small chart updates might break the lookup.
- Option 1 will work if the image is moved to another URI (e.g. for caching), option 2 will break unless the hardcoded image URI is updated as well.
- Neither method will work if post configuration changes image URI. E.g. a `kpt-set` of the image tag.
- With option 2 we can easily detect if there are image URI not covered by a `imageRegexp`. This is not possible with option 1.
