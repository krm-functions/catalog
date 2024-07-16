# Kubeconform

The `kubeconform` function wraps the
[kubeconform](https://github.com/yannh/kubeconform) manifest
validation tool.

Example functon-config:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-func-config
data:
  kubernetes_version: "1.30.0" // Must be one from https://github.com/instrumenta/kubernetes-json-schema without leading `v` e.g. `1.29.1`. Defaults to `master`
  ignore_missing_schemas: "true"
  strict: "true"
  schema_locations: "/path/to/schemas,/another/path"
```

For settings `schema_locations`, see [kubeconform docs](https://github.com/yannh/kubeconform#overriding-schemas-location).
