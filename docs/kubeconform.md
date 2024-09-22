# Kubeconform

The `kubeconform` function wraps the
[kubeconform](https://github.com/yannh/kubeconform) manifest
validation tool and can be used both imperatively and
declaratively. When run declaratively, the built-in schemas are used.

Example

```shell
export KUBECONFORM_IMAGE=ghcr.io/krm-functions/kubeconform@sha256:9fa8753fc17fc03eea6d2b63c9d8224cd7409a251820e511a3523ba55c5c9fe6

kpt fn source examples/kubeconform \
  | kpt fn eval - --truncate-output=false --image $KUBECONFORM_IMAGE -- ignore_missing_schemas=true
```

This command generates output like:

```
  Results:
    [info]: ConfigMap/valid
    [error] v1/ConfigMap/invalid-nested-dict configmap.yaml: /data/nested: expected string or null, but got object
    [error] v1/ConfigMap/invalid-non-string-value configmap.yaml: /data/a-number: expected string or null, but got number
    [error] external-secrets.io/v1beta1/ExternalSecret/example externalsecret.yaml: /spec: additionalProperties 'xXXsecretStoreRef' not allowed
    [error] gateway.networking.k8s.io/v1/Gateway/gateway-api-example-ns1/foo-gateway gateway.yaml: /spec: missing properties: 'gatewayClassName'
    [error] gateway.networking.k8s.io/v1/Gateway/gateway-api-example-ns1/foo-gateway gateway.yaml: /spec: additionalProperties 'xXXgatewayClassName' not allowed
    [error] karpenter.sh/v1beta1/NodePool/default karpenter-nodepool.yaml: /spec: missing properties: 'template'
    [error] karpenter.sh/v1beta1/NodePool/default karpenter-nodepool.yaml: /spec: additionalProperties 'xxtemplate' not allowed
    [error] karpenter.k8s.aws/v1beta1/EC2NodeClass/default karpenter-nodepool.yaml: /spec: missing properties: 'amiFamily'
    [error] karpenter.k8s.aws/v1beta1/EC2NodeClass/default karpenter-nodepool.yaml: /spec: additionalProperties 'xxamiFamily' not allowed
    [info]: Stats: {Resources:7 Invalid:6 Errors:0 Skipped:0}
```

# Schemas

The `kubeconform` KRM function can be used imperatively and support
external schemas through the `schema_locations` argument, which
follows the [schema_location for
kubeconform](https://github.com/yannh/kubeconform#overriding-schemas-location).

Some schemas are bundled into the function container - in most cases
these will be sufficient. See
[`source-schemas.sh`](scripts/source-schemas.sh) for which schemas are
included. The function can only be used declaratively with the
built-in schemas.

# function-config

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-kubeconform-config
data:
  kubernetes_version: "1.30.0"   // Must be one from https://github.com/instrumenta/kubernetes-json-schema without leading `v` e.g. `1.29.1`.
                                 // Defaults to `master`, which work with built-in schemas
  ignore_missing_schemas: "true" // Do not fail on missing schemas, only warn
  strict: "true"                 // Do not allow properties not defined in the schema
  schema_locations: "/path/to/schemas,/another/path"
```

For settings `schema_locations`, see [kubeconform docs](https://github.com/yannh/kubeconform#overriding-schemas-location).
