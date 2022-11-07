# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

This file itself is based on [Keep a CHANGELOG](https://keepachangelog.com/en/0.3.0/).

## [Unreleased]

### Added
- feat: add `args` for `controller`, `node`, `scheduler`, `lvmd` to helm chart (#576)

## [9.1.0] - 2022-10-04
### Added
- fix: add lvmd env option to helm chart (#563)

### Changed
- appVersion was changed to 0.15.2.
- build(deps): bump helm/chart-testing-action from 2.2.1 to 2.3.0 (#558)
- build(deps): bump helm/chart-releaser-action from 1.4.0 to 1.4.1 (#570)
- build(deps): bump helm/chart-testing-action from 2.3.0 to 2.3.1 (#572)

### Contributors
- @Davincible

## [9.0.1] - 2022-08-17
### Changed
- appVersion was changed to 0.15.1.

## [9.0.0] - 2022-08-16
### Changed
- support Kubernetes 1.24 (#529)
- Drop a PVC finalizer to delete pods (#536)
  - **BREAKING**: `pvcMutatingWebhook` setting is deleted from `charts/topolvm/values.yaml`.
- appVersion was changed to 0.15.0.

## [8.0.1] - 2022-07-05
### Changed
- appVersion was changed to 0.14.1.

## [8.0.0] - 2022-07-04
### Changed
- Removed: Inline Ephemeral Volume (#494)
  - **BREAKING**: Inline Ephemeral Volume is no longer supported.
- Remove helm install test. (#499)
- Bump helm/chart-releaser-action from 1.2.1 to 1.4.0 (#513)

### Contributors
- @bells17

## [7.0.0] - 2022-06-20
### Changed
- Disable a PDB for daemonset scheduler (#491)
- appVersion was changed to 0.13.0.

### Added
- Add support for creation of thin-snapshots (#463)
  - **BREAKING**: As of the release, the helm chart requires appVersion 0.13.0 or higher.

### Contributors
- @pluser
- @Yuggupta27
- @nbalacha

## [6.0.1] - 2022-06-06
### Changed
- appVersion was changed to 0.12.0.

## [6.0.0] - 2022-05-10

### Changed
- Add component labels (#470)
  - **BREAKING**: This PR changed the type of `controller.affinity` and the labels.
- appVersion was changed to 0.11.1.

### Contributors
- @bells17

## [5.0.0] - 2022-04-18

### Changed
- Modified to use ghcr.io as a container registry (#464)
- Updated the controller readiness probe endpoint (#469)
  - **BREAKING**: This PR supported `/readyz` endpoint which was introduced at topolvm 0.11.0. So topolvm 0.11.0 or later is required.

### Contributors
- @bells17

## [4.0.3] - 2022-04-04
### Fix
- No cert-manager CRs when webhook.caBundle is set (#451)

### Contributors
- @ooraini

## [4.0.2] - 2022-03-03
### Changed
- appVersion was changed to 0.10.6.

## [4.0.1] - 2022-02-04
### Changed
- appVersion was changed to 0.10.5.

### Added
- add readinessProbe for scheduler (#427)

## [4.0.0] - 2022-01-07

### Changed
- remove k8s version specification from Chart.yaml (#403)
- Make kubelet work directory overridable via single chart parameter (#410)
  - **BREAKING**: The `node.kubeletWorkDirectory` parameter has been added, and the default values of other parameters regarding the host path have changed. Please review the settings related to the host path.
- skip PDB when topolvm-scheduler isn't enabled (#417)

### Added
- Add topolvm-controller CLI flag to skip node finalize (#409)

### Contributors
- @macaptain
- @rkrzewski

## [3.2.0] - 2021-12-01
### Changed
- appVersion was changed to 0.10.3.

## [3.1.2] - 2021-11-01
### Changed
- appVersion was changed to 0.10.2.

### Added
- support pre and patch releases of k8s (#382)

### Contributors
- @sp98

## [3.1.1] - 2021-10-18

### Changed
- appVersion was changed to 0.10.1.

### Added

- support priorityClassName (#377)
- support PodMonitor (#373)
- support existingCertManagerIssuer (#372)

### Contributors
- @dungdm93

## [3.1.0] - 2021-09-30

### Changed
- add PDB/updateStrategy/priorityClass (#370)

## [3.0.0] - 2021-09-13

### Changed
- Change license to Apache License Version 2.0. (#360)
- appVersion was changed to 0.10.0.

## [2.1.1] - 2021-09-07

### Changed
- Fix lvmd is not previleged in deploying with Helm (#358)
- appVersion was changed to 0.9.2.

### Misc
- duplicate label causes YAML parsing errors (#351)

### Contributors
- @faruryo
- @khrisrichardson

## [2.1.0] - 2021-08-20

### Changed
- Make allowedHostPaths property of node's PSP configurable (#347)
- Update appVersion to 0.9.0 in Chart.yaml (#348)

### Contributors
- @debackerl

## [2.0.3] - 2021-08-19

### Removed
- Remove kustomize. (#336)

### Added
- Helm Chart: Support custom clusterIP for scheduler deployment (#346)

### Contributors
- @d-kuro
- @yuseinishiyama

## [2.0.1] - 2021-07-27

This is the first release.

[Unreleased]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v9.1.0...HEAD
[9.1.0]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v9.0.1...topolvm-chart-v9.1.0
[9.0.1]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v9.0.0...topolvm-chart-v9.0.1
[9.0.0]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v8.0.1...topolvm-chart-v9.0.0
[8.0.1]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v8.0.0...topolvm-chart-v8.0.1
[8.0.0]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v7.0.0...topolvm-chart-v8.0.0
[7.0.0]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v6.0.1...topolvm-chart-v7.0.0
[6.0.1]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v6.0.0...topolvm-chart-v6.0.1
[6.0.0]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v5.0.0...topolvm-chart-v6.0.0
[5.0.0]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v4.0.3...topolvm-chart-v5.0.0
[4.0.3]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v4.0.2...topolvm-chart-v4.0.3
[4.0.2]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v4.0.1...topolvm-chart-v4.0.2
[4.0.1]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v4.0.0...topolvm-chart-v4.0.1
[4.0.0]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v3.2.0...topolvm-chart-v4.0.0
[3.2.0]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v3.1.2...topolvm-chart-v3.2.0
[3.1.2]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v3.1.1...topolvm-chart-v3.1.2
[3.1.1]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v3.1.0...topolvm-chart-v3.1.1
[3.1.0]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v3.0.0...topolvm-chart-v3.1.0
[3.0.0]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v2.1.1...topolvm-chart-v3.0.0
[2.1.1]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v2.1.0...topolvm-chart-v2.1.1
[2.1.0]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v2.0.3...topolvm-chart-v2.1.0
[2.0.3]: https://github.com/topolvm/topolvm/compare/topolvm-chart-v2.0.1...topolvm-chart-v2.0.3
[2.0.1]: https://github.com/topolvm/topolvm/releases/tag/topolvm-chart-v2.0.1
