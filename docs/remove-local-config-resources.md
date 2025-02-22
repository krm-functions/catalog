# Remove local config resources Function

Resources which are meant as input for functions and not intended for final
output can be removed by this function by adding the annotation
`config.kubernetes.io/local-config: true`.

## Example

Using this input:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
data:
  foo1: bar1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
  annotations:
    config.kubernetes.io/local-config: true
data:
```

Apply the `remove-local-config-resources` function:

```shell
export REMOVE_LOCAL_CONFIG_RESOURCES_IMAGE=ghcr.io/krm-functions/remove-local-config-resources@sha256:081b5573300121efb9191cb5cef4a5d6d0c273f53d2a5ab6f889922f49607565

kpt fn source examples/remove-local-config-resources | \
    kpt fn eval - --results-dir tmp-results --truncate-output=false --i $REMOVE_LOCAL_CONFIG_RESOURCES_IMAGE -o unwrap
[RUNNING] "remove-local-config-resources"
[PASS] "remove-local-config-resources" in 100ms
  Results:
    [info]: remove ConfigMap/cm2

For complete results, see tmp-results/results.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
data:
  foo1: bar1
```

Notice, that `ConfigMap` `cm2` has been removed from the output.
