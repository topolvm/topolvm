# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

This file itself is based on [Keep a CHANGELOG](https://keepachangelog.com/en/0.3.0/).

## [Unreleased]

## [0.6.0] - 2020-09-25

### Fixed

- Fix default configuration file path (#202)

### Changed
- Replace base image (#199)

### Added
- Support for Kubernetes 1.18 (#204)
- Add Vagrant example (#183)

### Removed
- Support for Kubernetes 1.15 (#204)

### Contributors

- @frederiko

## [0.5.3] - 2020-08-12

### Changed

- Fix to accept implicit StorageClassName (#182)
- Fix documents (#172, #173, #174, #175, #176, #179)

### Contributors

- @briantopping
- @chez-shanpu
- @sebgl

## [0.5.2] - 2020-07-28

### Changed

- Change container repository (#170)

## [0.5.1] - 2020-07-22

### Changed

- Allow non-root container to use filesystem volume (#162)

## [0.5.0] - 2020-06-22

### Changed

- lvmd
  - **BREAKING**: Introduce device-class configuration file, instead of `--volume-group`, `--spare` and `--listen` options
  - Enhance gRPC interfaces of `lvmd` to support multiple volume groups

- topolvm-scheduler
  - **BREAKING**: Introduce the configuration file, instead of `--listen` and `--divisor` options

### Added

- Support for multiple volume groups (#147)

## [0.5.0-rc.1] - 2020-06-15

### Changed

- lvmd
  - **BREAKING**: Introduce device-class configuration file, instead of `--volume-group` and `--spare` options
  - Enhance gRPC interfaces of `lvmd` to support multiple volume groups

- topolvm-scheduler
  - **BREAKING**: Introduce the configuration file, instead of `--listen` and `--divisor` options

### Added

- Support for multiple volume groups (#147)

## [0.4.8] - 2020-05-28

### Fixed

- Recreate device file when expanding volume if needed (#144).

## [0.4.7] - 2020-05-08

### Changed

- Add key usages to certificate resources in sample manifests (#137).

### Added

- Add a design document about multiple volume groups (#131).

## [0.4.6] - 2020-04-21

### Changed

- Update client-go and controller-runtime (#135).

## [0.4.5] - 2020-04-16

Nothing changed.

## [0.4.4] - 2020-04-15

### Fixed

- LV name duplicates (#126).

## [0.4.3] - 2020-04-07

Nothing changed.

## [0.4.2] - 2020-04-03

### Changed
- Set default value for option `--leader-election-id` (#121).

## [0.4.1] - 2020-03-06

### Changed
- Upgrade for Kubernetes 1.17 (#115).
- topolvm-controller requires `--leader-election-id` flag.

## [0.4.0] - 2020-03-04

### Added
- Implement Volume expansion functionality (#101).
- Add scheduler tuning guide (#106).
- Deploy guide for Rancher/RKE (#108).

### Contributors
- @funkypenguin

## [0.3.0] - 2020-02-17

### Added
- Add support for volume tags to lvmd (#86).
- Add support for inline ephemeral volume (#93).

### Changed
- Upgrade cybozu-go/well to 1.10.0 (#85).
- Extend the timeout for waiting for the startup topolvm-controller (#90).
- Update CSIDriver config for k8s 1.16 for e2e while leaving legacy alone (#89).
- Update kubebuilder and controller-tools (#95).
- Change the author line to  "The TopoLVM Authors" (#98).

### Fixed
- Fix to allow creating Pods before their PVCs (#99).

### Contributors
- @matthias50
- @pohly
- @ridv

## [0.2.2] - 2019-12-26

Only cosmetic changes.

## [0.2.1] - 2019-12-17

### Changed
- Upgrade to support k8s 1.16 (#77)

## [0.2.0] - 2019-10-08

### Added
- Volumes and associated Pods are cleaned up after Node deletion (#53).
- Leader election of controller services (#58).
- Prometheus metrics for VG free space (#59, #63).
- Health checks for plugins (#61).
- Metrics for volume usage (bytes/inodes) (#62).
- `topolvm-controller` replaces `csi-topolvm` as CSI controller plugin.
- Official way of protecting namespaces from TopoLVM webhook (#57, #60).

### Changed
- Fix a bug in webhook (#54).

### Removed
- `topolvm-hook` is removed.  Its functions are merged into `topolvm-controller`.
- `lvmetrics` is removed.  Its functions are merged into `topolvm-node`.

## [0.1.2] - 2019-09-10

### Changed
- Update kubebuilder, controller-tools, controller-runtime (#35).
- Fix a bug in CSI GetCapacity method (#45).

## [0.1.1] - 2019-08-22

### Added
- A quick example to run TopoLVM on [kind](https://github.com/kubernetes-sigs/kind) (#18).
- A deployment tutorial (#18).

### Changed
- Re-implement `topolvm-hook` using Kubebuilder v2 (#19, #21).
- Update sidecar containers for Kubernetes 1.15 (#29).
- Update kubebuilder, controller-tools, controller-runtime, gRPC, client-go (#17, #24, #28).
- filesystem: stabilize mount point detection (#23).

## [0.1.0] - 2019-07-11

This is the first release.

[Unreleased]: https://github.com/topolvm/topolvm/compare/v6.0.0...HEAD
[0.6.0]: https://github.com/topolvm/topolvm/compare/v0.5.3...v6.0.0
[0.5.3]: https://github.com/topolvm/topolvm/compare/v0.5.2...v0.5.3
[0.5.2]: https://github.com/topolvm/topolvm/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/topolvm/topolvm/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/topolvm/topolvm/compare/v0.5.0-rc.1...v0.5.0
[0.5.0-rc.1]: https://github.com/topolvm/topolvm/compare/v0.4.8...v0.5.0-rc.1
[0.4.8]: https://github.com/topolvm/topolvm/compare/v0.4.7...v0.4.8
[0.4.7]: https://github.com/topolvm/topolvm/compare/v0.4.6...v0.4.7
[0.4.6]: https://github.com/topolvm/topolvm/compare/v0.4.5...v0.4.6
[0.4.5]: https://github.com/topolvm/topolvm/compare/v0.4.4...v0.4.5
[0.4.4]: https://github.com/topolvm/topolvm/compare/v0.4.3...v0.4.4
[0.4.3]: https://github.com/topolvm/topolvm/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/topolvm/topolvm/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/topolvm/topolvm/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/topolvm/topolvm/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/topolvm/topolvm/compare/v0.2.2...v0.3.0
[0.2.2]: https://github.com/topolvm/topolvm/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/topolvm/topolvm/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/topolvm/topolvm/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/topolvm/topolvm/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/topolvm/topolvm/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/topolvm/topolvm/compare/8d34ac6690b0326d1c08be34f8f4667cff47e9c0...v0.1.0
