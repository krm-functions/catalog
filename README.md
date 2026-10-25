# KRM Functions

This repository contain
[KRM functions](https://medium.com/@michael.vittrup.larsen/replacing-helm-and-kustomize-with-krm-functions-a-new-approach-to-configuration-management-676212cc1332):

See [krm-functions organisation](https://github.com/krm-functions) for a list of
functions available.

## Building

All build tools are pinned in [devbox.json](devbox.json):

```
devbox shell
make build           # binaries in bin/<os>_<arch>/
make lint test
make e2e-tests       # some tests run the function images and need docker/podman
make push           # build and push multi-arch images with ko (REGISTRY=...)
```

Images are built with [ko](https://ko.build). The Helm-based functions use a
base image built with [apko](https://github.com/chainguard-dev/apko) (see
`build/base-helm.apko.yaml`), the kubeconform schema bundle needs
`scripts/source-schemas.sh` to be run first.
