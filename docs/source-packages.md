# Source Packages Function

The `source-packages` function implements a declarative solution for [`kpt`](https://kpt.dev/book/03-packages/) packages.

If you manage fleets of packages with a number of invocations of `kpt pkg get` like:

```shell
kpt pkg get https://example.git/package1@v1.0
kpt pkg get https://example.git/package2@v1.1
kpt pkg get https://example.git/package3@v1.2
```

Then the `source-packages` function allows you to specify this declaratively using a `Fleet` resource:

```yaml
apiVersion: foo.bar
kind: Fleet
metadata:
  name: example-fleet
spec:
  # These settings can also be given individually for each package
  defaults:
    upstream:
      type: git
      git:
        repo: https://example.git
        ref: main

  packages:
  - name: foo
    sourcePath: pkg1
  - name: bar
    sourcePath: pkg2
    packages:     # Package definitions can be recursive
    - name: baz
      sourcePath: pkg3
```

This example will source packages from `https://example.git@main` and
create the following package structure:

```shell
foo/
  <files from 'https://example.git@main/pkg1'>
bar/
  <files from 'https://example.git@main/pkg2'>
  baz/
    <files from 'https://example.git@main/pkg3'>
```

Recursive packages is very convenient for composing a package from
several sub-packages. This is similar to how the [Open Application
Model](https://oam.dev/) handles
[traits](https://github.com/oam-dev/spec/blob/master/6.traits.md).
