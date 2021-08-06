Release procedure
=================

This document describes how to release a new version of TopoLVM.

Versioning
----------

Follow [semantic versioning 2.0.0][semver] to choose the new version number.

Prepare change log entries
--------------------------

Add notable changes since the last release to [CHANGELOG.md](CHANGELOG.md).
It should look like:

```markdown
(snip)
## [Unreleased]

### Added
- Add a notable feature for users (#35)

### Changed
- Change a behavior affecting users (#33)

### Removed
- Remove a feature, users action required (#39)

### Fixed
- Fix something not affecting users or a minor change (#40)

### Contributors
- @hoge
- @foo

(snip)
```

Bump version
------------

1. Determine a new version number.  Export it as an environment variable:

    ```console
    $ VERSION=1.2.3
    $ export VERSION
    ```

2. Checkout `main` branch.
3. Make a branch to release, for example by `git neco dev bump-$VERSION`
4. Update image versions in files below:
   - charts/topolvm/Chart.yaml
   - example/Makefile
   - example/README.md
    ```console
    $ sed -r -i "s/appVersion: [[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+/appVersion: ${VERSION}/g" charts/topolvm/Chart.yaml
    $ sed -r -i "s/TOPOLVM_VERSION=[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+/TOPOLVM_VERSION=${VERSION}/g" example/Makefile
    $ sed -r -i "s/checkout v[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+/checkout v${VERSION}/g" example/README.md
    ```
5. Edit `CHANGELOG.md` for the new version ([example][]).
6. Commit the change and create a pull request:

    ```console
    $ git commit -a -m "Bump version to $VERSION"
    $ git neco review
    ```

7. Merge the new pull request.
8. Add a new tag and push it as follows:

    ```console
    $ git checkout main
    $ git pull
    $ git tag v$VERSION
    $ git push origin v$VERSION
    ```

Publish GitHub release page
---------------------------

Once a new tag is pushed to GitHub, [GitHub Actions][] automatically
builds a tar archive for the new release, and uploads it to GitHub
releases page.

Visit https://github.com/topolvm/topolvm/releases to check
the result.  You may manually edit the page to describe the release.

Release Helm Chart
------------------

TopoLVM Helm Chart will be released independently.
This will prevent the TopoLVM version from going up just by modifying the Helm Chart.

You must change the version of [Chart.yaml](./charts/topolvm/Chart.yaml) when making changes to the Helm Chart.
CI fails with lint error when creating a Pull Request without changing the version of [Chart.yaml](./charts/topolvm/Chart.yaml).

When you release the Helm Chart, manually run the GitHub Actions workflow for the release.

https://github.com/topolvm/topolvm/actions/workflows/helm-release.yaml

When you run workflow, [helm/chart-releaser-action](https://github.com/helm/chart-releaser-action) will automatically create a GitHub Release.

[semver]: https://semver.org/spec/v2.0.0.html
[example]: https://github.com/cybozu-go/etcdpasswd/commit/77d95384ac6c97e7f48281eaf23cb94f68867f79
[GitHub Actions]: https://github.com/topolvm/topolvm/actions
