# Set-annotations Function

The `set-annotations` function adds annotations to resources.

Annotations are defined through function-config either as a Configmap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  foo: bar
  baz: olo
```

or as a dedicated `SetAnnotations` resource:

```yaml
apiVersion: fn.kpt.dev/v1alpha1
kind: SetAnnotations
metadata:
  name: test-set-annotations
annotations:
  foo: bar
  baz: olo
```
