# KRM Functions

This repository have two [KRM functions](https://medium.com/@michael.vittrup.larsen/replacing-helm-and-kustomize-with-krm-functions-a-new-approach-to-configuration-management-676212cc1332):

- [helm-upgrader](docs/helm-upgrader.md) Function for automating
  upgrades of Helm chart specifications in e.g. KRM `RenderHelmChart`
  format. Supports upgrade constraints.
- [render-helm-chart](docs/render-helm-chart.md) A variant of the
  [baseline
  `render-helm-chart`](https://catalog.kpt.dev/render-helm-chart/v0.2/)
  that can be used in [declarative
  pipelines](https://kpt.dev/book/04-using-functions/01-declarative-function-execution)
  through Kptfiles.
