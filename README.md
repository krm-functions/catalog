# KRM Functions

This repository contain [KRM functions](https://medium.com/@michael.vittrup.larsen/replacing-helm-and-kustomize-with-krm-functions-a-new-approach-to-configuration-management-676212cc1332):

- [helm-upgrader](docs/helm-upgrader.md) Function for automating
  upgrades of Helm chart specifications in e.g. KRM `RenderHelmChart`
  format. Supports upgrade constraints.
- [render-helm-chart](docs/render-helm-chart.md) A re-implementation
  of the [baseline
  `render-helm-chart`](https://catalog.kpt.dev/render-helm-chart/v0.2/)
  function, which can be used in [declarative
  pipelines](https://kpt.dev/book/04-using-functions/01-declarative-function-execution)
  through Kptfiles. :warning: note, this function is dual purpose, it
  can both source and render charts. The sourcing feature is
  deprecated - use the [source-helm-chart](docs/source-helm-chart.md)
  for this instead.
- [source-helm-chart](docs/source-helm-chart.md) A function that
  implements sourcing of helm charts to support rendering using
  declarative pipelines. This function augments
  [render-helm-chart](docs/render-helm-chart.md).
- [apply-setters](docs/apply-setters.md) A re-implementation and
  improvement of the [baseline
  `apply-setters`](https://catalog.kpt.dev/apply-setters/v0.2/)
  function, which supports merge of multiple sources of apply-setters
  configuration and accepts configuration through both function-config
  and primary resource list. Also supports reading setter values from
  other resources.
- [gatekeeper](https://github.com/michaelvl/krm-gatekeeper) A
  re-implementation of the [baseline
  `gatekeeper`](https://catalog.kpt.dev/gatekeeper/v0.2/) function,
  which suppors newer variants of the Rego language (e.g. as used in
  the
  [gatekeeper-library](https://github.com/open-policy-agent/gatekeeper-library))
  and which support gatekeeper
  [expansions](https://open-policy-agent.github.io/gatekeeper/website/docs/expansion)
- [digester](docs/digester.md) a function that can lookup container
  image digests for Helm charts and write-back digests into chart
  values.
