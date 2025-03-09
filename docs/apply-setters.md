# Apply-Setters Function

The `apply-setters` KRM function provides two extensions to
the [baseline
apply-setters](https://catalog.kpt.dev/apply-setters/v0.2/) function:

- It accepts one or more `ApplySetters` resource(s) through the main
  resource input or as function-config. This means setters values can
  be manipulated by a pipeline of KRM functions and that setters from
  multiple sources can be integrated.
- Setters can take values from other resources through a field-path.

The example `ApplySetters` resource below illustrates this:

```yaml
apiVersion: fn.kpt.dev/v1alpha1
kind: ApplySetters
metadata:
  name: inline-setters-spec
  annotations:
    config.kubernetes.io/local-config: true
setters:
  # These work like traditional ConfigMap-based apply-setters function-config
  data:
    bar: valueBar
    baz: valueBaz

  # These references resources, and reads from a field path and turns read
  # data into a setters value . This is similar to the
  # apply-replacements KRM function.
  references:
    - setterName: deployReplicas # Use the value from source below as setter with this name
      source:
        kind: Deployment # A resource to locate
        name: a-deployment
        fieldPath: spec.replicas # Read this field from resource

    - setterName: kptGitSha
      source:
        kind: Kptfile
        fieldPath: upstream.git.ref
```

## Configuration Management

Configuration management is the process of configuring generic
components for a specific use-case or environment.

The two common methods for configuration management, i.e. adding or
modifying Kubernetes resources, available in the curated library of
KRM functions are the
[apply-setters](https://catalog.kpt.dev/apply-setters/v0.2/) and
[apply-replacements](https://catalog.kpt.dev/apply-replacements/v0.1/)
functions.

There are pros and cons of both methods.

The apply-setters function allows simple configuration of scalars,
with the configuration provided through KRM function-config.The
benefit of the apply-setters function is that the replacements are
specified directly in the resource files with `# kpt-set: ...`
comments. The disadvantage is, that the replacements are provided
through function-config and thus needs to be prepared prior to running
a render pipeline and cannot be modified by the pipeline itself.

If one require more advanced configuration, the `apply-replacements`
function can provide both replacement of non-scalars and
e.g. searching in lists. The `apply-replacements` function also
accepts replacement configuration through function-config, however
this 'replacement configuration' can reference resources that are
mutated by the render pipeline. The major disadvantage of the
`apply-replacements` function is that the replacements are defined
external to the Kubernetes resources modified and it can be cumbersome
to keep the list of replacements in sync with the resources that
should be modified.

A third option is the [value-propagation
pattern](https://kpt.dev/guides/value-propagation) that use the
`apply-replacements` function together with the [starlark
function](https://catalog.kpt.dev/starlark/v0.4/) to allow
code-defined preparation of the input for the `apply-repalcements`
function. This enables a wide range of modifications to be applied in
a render pipeline, but it still suffers from the disadvantages of the
`apply-replacements` function.

## Example Usage

```shell
kpt fn source examples/apply-setters \
 | kpt fn eval - --truncate-output=false --image ghcr.io/krm-functions/apply-setters \
    --fn-config example-function-configs/apply-setters/apply-setters-fn-config.yaml \
 | kpt fn eval - --image ghcr.io/krm-functions/remove-local-config-resources -o unwrap
```

Notice, how the `replicas` field from the `Deployment` is inserted
into the `ConfigMap` and the Git SHA from the `Kptfile` resource is
inserted into the `Deployment` `version` label:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: apply-setters-fn-config
data:
  ...
  replicas: "4" # kpt-set: ${deployReplicas}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: a-deployment
  namespace: olo
  labels:
    app.kubernetes.io/version: "a1b2c3d4e5e6" # kpt-set: ${kptGitSha}
spec:
  replicas: 4
...
```
