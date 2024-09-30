# Source Packages Function

The `source-packages` function implements a declarative solution for
[`kpt`](https://kpt.dev/book/03-packages/) packages similar to how
[helmfile](https://github.com/helmfile/helmfile) manages fleets of
Helm charts.

If you manage fleets of packages with a number of invocations of `kpt pkg get` like:

```shell
kpt pkg get https://example.git/package1@v1.0
kpt pkg get https://example.git/package2@v1.1
kpt pkg get https://example.git/package3@v1.2
```

The `source-packages` function allows you to specify this declaratively using a `Fleet` resource:

```yaml
apiVersion: fn.kpt.dev/v1alpha1
kind: Fleet
metadata:
  name: example-fleet
spec:
  upstreams:
  - name: example-upstream
    type: git
    git:
      repo: https://example.git

  packages:
  - name: package1       # similar to 'kpt pkg get https://example.git/package1@v1.0'
    sourcePath: package1
    upstream: example-upstream
    ref: v1.0

  - name: package2       # similar to 'kpt pkg get https://example.git/package2@v1.1'
    sourcePath: package2
    upstream: example-upstream
    ref: v1.1

  - name: package3       # similar to 'kpt pkg get https://example.git/package3@v1.2'
    sourcePath: package3
    upstream: example-upstream
    ref: v1.2
```

A `defaults` setion can be used to remove some repetition for common settings:

```yaml
apiVersion: fn.kpt.dev/v1alpha1
kind: Fleet
metadata:
  name: example-fleet
spec:
  upstreams:
  - name: example-upstream
    type: git
    git:
      repo: https://example.git

  # These settings can also be given individually for each package
  defaults:
    upstream: example-upstream
    ref: main

  # 'sourcePath' defaults to package 'name'
  packages:
  - name: package1
  - name: package2
  - name: package3
```

Packages can also be composed recursively:

```yaml
  ...
  packages:
  - name: foo
    sourcePath: pkg1
  - name: bar
    sourcePath: pkg2
    packages:     # Package definitions can be recursive, i.e. 'baz' is inside 'bar'
    - name: baz
      sourcePath: pkg3
```

This example will source packages from `https://example.git@main` and
create the following package structure:

```
example-fleet
├── foo/
│   └── <files from 'https://example.git@main/pkg1'>
└── bar/
    ├── <files from 'https://example.git@main/pkg2'>
    └── baz/
        └── <files from 'https://example.git@main/pkg3'>
```

Recursive packages is very convenient for composing a package from
several sub-packages. This is similar to how the [Open Application
Model](https://oam.dev/) handles
[traits](https://github.com/oam-dev/spec/blob/master/6.traits.md).

A useful example of this *package composition pattern* is if a common
pipeline (as defined in a `Kptfile`) should be applied to
sub-packages. In this case, the pipeline package can be used as
parent:

```yaml
  ...
  packages:
  - name: common-pipeline
    packages:   # common-pipeline `Kptfile` will be applied to sub-packages
    - name: foo
    - name: bar
```

Alternatively, package composition can be created using 'stub' tree
nodes, which is basically just a named directory for sub-packages:

```yaml
  ...
  packages:
  - name: top
    stub: true  # no 'sourcePath', instead it is explicitly marked as empty stub node
    packages:
    - name: sub1       # will be stored in 'top/sub1'
      sourcePath: pkg1
    - name: sub2       # will be stored in 'top/sub2'
      sourcePath: pkg2
```

Note, that no package merge strategies are supported.

## Example Usage

```shell
export SOURCE_PACKAGES_IMAGE=ghcr.io/krm-functions/source-packages@sha256:30e52b8976e867d50d0a1745e2577c806790987befb477e3ca8ea53bd0aa3859

kpt fn source examples/source-packages/specs | \
  kpt fn eval - --network -i $(SOURCE_PACKAGES_IMAGE) | \
  kpt fn sink fn-output

kpt pkg tree fn-output
```

## Private Repositories/Upstreams

Private repositories are supported through SSH-agent integration:

```yaml
  upstreams:
  - name: example-upstream
    type: git
    git:
      repo: git@github.com:example-org/example-repo.git
      authMethod: sshAgent
```

The SSH-agent socket must be mounted into the container:

```shell
export SOURCE_PACKAGES_IMAGE=ghcr.io/krm-functions/source-packages@sha256:30e52b8976e867d50d0a1745e2577c806790987befb477e3ca8ea53bd0aa3859

kpt fn source examples/source-packages/specs | \
  kpt fn eval - -e SSH_AUTH_SOCK --mount type=bind,src="$SSH_AUTH_SOCK",target="$SSH_AUTH_SOCK",rw=true --as-current-user --network -i $(SOURCE_PACKAGES_IMAGE) | \
  kpt fn sink fn-output
```

Private keys can also be used by referencing a `Secret` resource:

```yaml
  upstreams:
  - name: example-upstream
    type: git
    git:
      repo: git@github.com:example-org/example-repo.git
      authMethod: sshPrivateKey
      ssh:
        apiVersion: v1
        kind: Secret
        name: ssh-private-key
        namespace: optional-namespace
```

The `Secret` must have `ssh-username` and `ssh-privatekey`fields e.g.:

```
kubectl create secret generic foo --dry-run=client --type=kubernetes.io/ssh-auth \
  --from-literal ssh-username=git --from-file ssh-privatekey=<key-file> -o yaml
```

The container's `known_hosts` file currently contain GitHub SSH hosts
only. See the `ssh` folder.

## Package Metadata

The default behaviour of `source-packages` is to create a
`package-context.yaml` file together with the sourced package in a
way compatible with `kpt`, i.e. with a single `name` field:

```
# package-context.yaml in kpt compatible mode
apiVersion: v1
kind: ConfigMap
metadata:
  name: kptfile.kpt.dev
  annotations:
    config.kubernetes.io/local-config: "true"
data:
  name: <destination name of package>
```

The `source-packages` function allows additional metadata to be added
to the `package-context.yaml` file through the `metadata.spec` field:

```
  packages:
  - name: foo
    sourcePath: examples/source-packages/pkg1
    metadata:
      spec:
        k2: v2   # Additional key/values to be added to package-context.yaml
        k3: v3
```

Metadata can both be defaulted and inherited by sub-packages, i.e. the
following:

```
  defaults:
    metadata:
      spec:
        k1: v1
        k2: v2
  packages:
  - name: foo
    metadata:
      spec:
        k2: v2-2
        k3: v3-2
    packages:
    - name: foo-sub-pkg
      metadata:
        spec:
          k3: v3-3
          k4: v4-3
```

will result in the following `package-context.yaml` for the
`foo-sub-pkg` package:

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: kptfile.kpt.dev
  annotations:
    config.kubernetes.io/local-config: "true"
data:
  name: foo-sub-pkg
  k1: v1    # From 'defaults'
  k2: v2-2  # Inherited from parent 'foo'
  k3: v3-3  # foo-sub-pkg value takes precedence over parent value
  k4: v4-3
```

## Future Directions

- Currently, `source-packages` is not recursive and `Fleet` resources
  fetched as part of a package is not processed.
- Generally, better error checking could be implemented
- OCI upstreams
