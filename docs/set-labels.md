# Set-labels Function

The `set-labels` function adds labels to resources.

Labels are defined through function-config either as a Configmap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-config
data:
  foo: bar
  baz: olo
```

or as a dedicated `SetLabels` resource:

```yaml
apiVersion: fn.kpt.dev/v1alpha1
kind: SetLabels
metadata:
  name: test-set-labels
labels:
  foo: bar
  baz: olo
```

This function does not set selector labels. An optional `setSelectorLabels`
parameter may be given but only `false` will be accepted. This may change
in future versions:

```yaml
apiVersion: fn.kpt.dev/v1alpha1
kind: SetLabels
metadata:
  name: test-set-labels
labels:
  foo: bar
  baz: olo
setSelectorLabels: false
```
