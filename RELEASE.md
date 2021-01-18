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
- Implement ... (#35)

### Changed
- Fix a bug in ... (#33)

### Removed
- Deprecated `-option` is removed ... (#39)

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

2. Checkout `master` branch.
3. Make a branch to release, for example by `git neco dev bump-$VERSION`
4. Update image versions in files below:
   - deploy/manifests/overlays/deployment-scheduler/kustomization.yaml
   - deploy/manifests/overlays/daemonset-scheduler/kustomization.yaml
   - example/Makefile
    ```console
    $ sed -r -i "s/newTag: [[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+/newTag: ${VERSION}/g" deploy/manifests/overlays/deployment-scheduler/kustomization.yaml deploy/manifests/overlays/daemonset-scheduler/kustomization.yaml
    $ sed -r -i "s/TOPOLVM_VERSION=[[:digit:]]+\.[[:digit:]]+\.[[:digit:]]+/TOPOLVM_VERSION=${VERSION}/g" example/Makefile
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
    $ git checkout master
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

[semver]: https://semver.org/spec/v2.0.0.html
[example]: https://github.com/cybozu-go/etcdpasswd/commit/77d95384ac6c97e7f48281eaf23cb94f68867f79
[GitHub Actions]: https://github.com/topolvm/topolvm/actions
