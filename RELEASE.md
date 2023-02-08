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

2. Make a branch for the release as follows:

    ```console
    $ git checkout main
    $ git pull
    $ git checkout -b bump-$VERSION
    ```

3. Edit `CHANGELOG.md` for the new version ([example][]).
   The candidate of relevant PRs can be obtained by the following command.
   ```
   git log --merges --format="%s%x00%b" $(git tag | grep "^v" | sort -V -r | head -n 1)..main | sed -E 's|^Merge pull request #([0-9]*)[^\x0]*\x0(.*)|- \2 ([#\1](https://github.com/topolvm/topolvm/pull/\1))|' | tac
   ```
   Please remove PRs which contain changes only to the helm chart.

4. Commit the change and create a pull request:

    ```console
    $ git commit -a -m "Bump version to $VERSION"
    $ git push -u origin bump-$VERSION
    ```

5. Create new pull request and merge it.
6. Add a new tag and push it as follows:

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

Bump Chart Version
------------------

TopoLVM Helm Chart will be released independently.
This will prevent the TopoLVM version from going up just by modifying the Helm Chart.

1. Determine a new version number.  Export it as an environment variable:

    ```console
    $ APPVERSION=1.2.3
    $ export APPVERSION
    $ CHARTVERSION=4.5.6
    $ export CHARTVERSION
    ```

2. Make a branch for the release as follows:

    ```console
    $ git checkout main
    $ git pull
    $ git checkout -b bump-chart-$CHARTVERSION
    ```

3. Update image versions in files below:
   - example/Makefile
   - example/README.md
   - charts/topolvm/Chart.yaml
    ```console
    $ sed -r -i "s/TOPOLVM_VERSION := [[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+/TOPOLVM_VERSION := ${APPVERSION}/g" example/Makefile
    $ sed -r -i "s/checkout v[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+/checkout v${APPVERSION}/g" example/README.md
    $ sed -r -i "s/appVersion: [[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+/appVersion: ${APPVERSION}/g" charts/topolvm/Chart.yaml
    $ sed -r -i "s/^version: [[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+/version: ${CHARTVERSION}/g" charts/topolvm/Chart.yaml
    $ sed -r -i "s/ghcr.io\/topolvm\/topolvm-with-sidecar:[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+/ghcr.io\/topolvm\/topolvm-with-sidecar:${APPVERSION}/g" charts/topolvm/Chart.yaml
    $ sed -r -i "s/ghcr.io\/topolvm\/topolvm:[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+/ghcr.io\/topolvm\/topolvm:${APPVERSION}/g" charts/topolvm/Chart.yaml
    ```

4. Edit `charts/topolvm/CHANGELOG.md` for the new version ([example][]).
   The candidate of relevant PRs can be obtained by the following command.
   ```
   git log --merges --format="%s%x00%b" $(git tag | grep "^topolvm-chart-v" | sort -V -r | head -n 1)..main | sed -E 's|^Merge pull request #([0-9]*)[^\x0]*\x0(.*)|- \2 ([#\1](https://github.com/topolvm/topolvm/pull/\1))|' | tac
   ```
   Please select PRs which contain changes to the helm chart.

5. Commit the change and create a pull request:

    ```console
    $ git commit -a -m "Bump chart version to $CHARTVERSION"
    $ git push -u origin bump-chart-$CHARTVERSION
    ```

6. Create new pull request and merge it.

7. Manually run the GitHub Actions workflow for the release.

   https://github.com/topolvm/topolvm/actions/workflows/helm-release.yaml

   When you run workflow, [helm/chart-releaser-action](https://github.com/helm/chart-releaser-action) will automatically create a GitHub Release.

[semver]: https://semver.org/spec/v2.0.0.html
[example]: https://github.com/cybozu-go/etcdpasswd/commit/77d95384ac6c97e7f48281eaf23cb94f68867f79
[GitHub Actions]: https://github.com/topolvm/topolvm/actions
