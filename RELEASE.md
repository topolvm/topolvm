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

(snip)
```

Bump version
------------

1. Determine a new version number.  Let it write `$VERSION` as `VERSION=x.y.z`.
2. Checkout `master` branch.
3. Make a branch to release, for example by `git neco dev "$VERSION"`
4. Update `version.go`.
5. Edit `CHANGELOG.md` for the new version ([example][]).
6. Commit the change and push it.

    ```console
    $ git commit -a -m "Bump version to $VERSION"
    $ git neco review
    ```

7. Merge this branch.
8. Checkout `master` branch.
9. Add a git tag, then push it.

    ```console
    $ git tag "v$VERSION"
    $ git push origin "v$VERSION"
    ```

Publish GitHub release page
---------------------------

Once a new tag is pushed to GitHub, [CircleCI][] automatically
builds a tar archive for the new release, and uploads it to GitHub
releases page.

Visit https://github.com/cybozu-go/etcdpasswd/releases to check
the result.  You may manually edit the page to describe the release.

[semver]: https://semver.org/spec/v2.0.0.html
[example]: https://github.com/cybozu-go/etcdpasswd/commit/77d95384ac6c97e7f48281eaf23cb94f68867f79
[CircleCI]: https://circleci.com/gh/cybozu-go/topolvm
