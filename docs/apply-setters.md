# Apply-Setters Function

TL;DR - this `apply-setters` KRM function provides two extensions to
the [baseline
apply-setters](https://catalog.kpt.dev/apply-setters/v0.2/) function:

- It accepts one or more `ApplySetters` resource(s) through the main
  resource input. This means setters values can be manipulated by a
  pipeline of KRM functions and that setters from multiple sources can
  be integrated.
- Setters can take values from other resources through a field-path.

The example `ApplySetters` resource below illustrates this:

```yaml
apiVersion: experimental.fn.kpt.dev/v1alpha1
kind: ApplySetters
metadata:
  name: inline-setters-spec
setters:
  # These work like traditional ConfigMap-based apply-setters function-config
  data:
    bar: valueBar
    baz: valueBaz

  # These references resources, and reads from a field path and turns read
  # data into a setters value . This is similar to the
  # apply-replacements KRM function.
  references:
  - source:
      kind: Deployment          # A resource to locate
      name: my-nginx
      fieldPath: spec.replicas  # Read this field from resource
    as: deployReplicas          # Use read value as setter with this name
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
