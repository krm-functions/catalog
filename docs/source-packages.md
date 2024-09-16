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
  packages:
  - name: package1       # similar to 'kpt pkg get https://example.git/package1@v1.0'
    sourcePath: package1
    upstream:
	  git:
        repo: https://example.git
        ref: v1.0

  - name: package2       # similar to 'kpt pkg get https://example.git/package2@v1.1'
    sourcePath: package2
    upstream:
	  git:
        repo: https://example.git
        ref: v1.1

  - name: package3       # similar to 'kpt pkg get https://example.git/package3@v1.2'
    sourcePath: package3
    upstream:
	  git:
        repo: https://example.git
        ref: v1.2
```

The `defaults` setion can be used to remove some repetition for common settings:

```yaml
apiVersion: fn.kpt.dev/v1alpha1
kind: Fleet
metadata:
  name: example-fleet
spec:
  # These settings can also be given individually for each package
  defaults:
    upstream:
      git:
        repo: https://example.git
        ref: main

  packages:
  - name: package1
    sourcePath: package1
  - name: package2
    sourcePath: package2
  - name: package3
    sourcePath: package3
```

Packages can also be composed in recursively:

```yaml
apiVersion: fn.kpt.dev/v1alpha1
kind: Fleet
metadata:
  name: example-fleet
spec:
  # These settings can also be given individually for each package
  defaults:
    upstream:
      git:
        repo: https://example.git
        ref: main

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
├── bar/
│   ├── <files from 'https://example.git@main/pkg2'>
    └── baz/
        └── <files from 'https://example.git@main/pkg3'>
```

Recursive packages is very convenient for composing a package from
several sub-packages. This is similar to how the [Open Application
Model](https://oam.dev/) handles
[traits](https://github.com/oam-dev/spec/blob/master/6.traits.md).
