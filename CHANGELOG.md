# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

This file itself is based on [Keep a CHANGELOG](https://keepachangelog.com/en/0.3.0/).

## [Unreleased]

## [0.22.0] - 2023-10-10

### Added

- feat: add option to enable/disable webhook server ([#749](https://github.com/topolvm/topolvm/pull/749))

### Changed

- build(deps): bump the github-actions-update group with 2 updates ([#756](https://github.com/topolvm/topolvm/pull/756))
- Replace cybozu/octoken-action with actions/create-github-app-token ([#755](https://github.com/topolvm/topolvm/pull/755))
- Refine exempt-issue-labels to ignore update kubernetes ([#753](https://github.com/topolvm/topolvm/pull/753))
- Support Kubernetes 1.27 ([#758](https://github.com/topolvm/topolvm/pull/758))

### Contributors

- @suleymanakbas91
- @toshipp
- @cupnes

## [0.21.0] - 2023-09-13

### Changed

- feat: allow snapshotting to greater target than source volume ([#738](https://github.com/topolvm/topolvm/pull/738))

### Fixed

- Fix typos. ([#739](https://github.com/topolvm/topolvm/pull/739))
- remove redundant explanation of snapshot limitation. ([#741](https://github.com/topolvm/topolvm/pull/741))

### Contributors

- @jakobmoellerdev
- @adelton
- @peng225

## [0.20.0] - 2023-08-08

### Added

- feat: allow passing leaderelection config values to topolvm-controller ([#728](https://github.com/topolvm/topolvm/pull/728))

### Changed

- Start building images independently of unit tests ([#719](https://github.com/topolvm/topolvm/pull/719))
- add Ryotaro Banno to owners ([#722](https://github.com/topolvm/topolvm/pull/722))
- Use pre-build cri-dockerd ([#725](https://github.com/topolvm/topolvm/pull/725))
- Use dependabot grouping feature ([#724](https://github.com/topolvm/topolvm/pull/724))

### Fixed

- Don't re-add finalizer to LogicalVolumes that are about to be deleted ([#723](https://github.com/topolvm/topolvm/pull/723))

### Contributors

- @jakobmoellerdev
- @toshipp
- @llamerada-jp
- @spmason

## [0.19.1] - 2023-07-07

### Changed

- Specify kind node image digest ([#705](https://github.com/topolvm/topolvm/pull/705))
- Add an item to the check list for Kubernetes upgrade to ensure that t… ([#708](https://github.com/topolvm/topolvm/pull/708))
- doc: update limitation about retain policy ([#714](https://github.com/topolvm/topolvm/pull/714))
- Stabilize logical volume test ([#706](https://github.com/topolvm/topolvm/pull/706))
- build(deps): bump google.golang.org/grpc from 1.49.0 to 1.53.0 ([#716](https://github.com/topolvm/topolvm/pull/716))

### Contributors

- @peng225
- @satoru-takeuchi
- @toshipp

## [0.19.0] - 2023-05-18

### Changed

- Refactor getting objects ([#679](https://github.com/topolvm/topolvm/pull/679))
- build(deps): bump actions/stale from 7 to 8 ([#688](https://github.com/topolvm/topolvm/pull/688))
- Unify LV check logic ([#685](https://github.com/topolvm/topolvm/pull/685))
- Combine capacity test and scheduler test ([#689](https://github.com/topolvm/topolvm/pull/689))
- Cleanup publish test ([#693](https://github.com/topolvm/topolvm/pull/693))
- Update cleanup test to check if topolvm re-create pods of STS if the node is deleted. ([#696](https://github.com/topolvm/topolvm/pull/696))
- Organize metrics tests ([#698](https://github.com/topolvm/topolvm/pull/698))
- Run thick and thin sanity at once ([#700](https://github.com/topolvm/topolvm/pull/700))
- support Kubernetes 1.26 ([#697](https://github.com/topolvm/topolvm/pull/697))

### Contributors

- @llamerada-jp
- @toshipp

## [0.18.2] - 2023-04-04

### Added

- Add tests for NodeReconciler ([#670](https://github.com/topolvm/topolvm/pull/670))
- add a workflow job to check the do-not-merge label ([#677](https://github.com/topolvm/topolvm/pull/677))

### Changed

- Use csi go module instead of generating sources from proto ([#669](https://github.com/topolvm/topolvm/pull/669))
- move legacy mode test to e2e test ([#665](https://github.com/topolvm/topolvm/pull/665))
- Refactor: use controllerutil to operate finalizers ([#671](https://github.com/topolvm/topolvm/pull/671))
- test the example only when the helm chart is released  ([#673](https://github.com/topolvm/topolvm/pull/673))
- Fill Status.CurrentSize ([#666](https://github.com/topolvm/topolvm/pull/666))
- ci: reuse topolvm.img in e2e test matrix ([#675](https://github.com/topolvm/topolvm/pull/675))
- Refactor e2e ([#678](https://github.com/topolvm/topolvm/pull/678))
- Remove hook test from e2e ([#684](https://github.com/topolvm/topolvm/pull/684))
- build(deps): bump helm/chart-testing-action from 2.3.1 to 2.4.0 ([#686](https://github.com/topolvm/topolvm/pull/686))
- build(deps): bump actions/setup-go from 3 to 4 ([#687](https://github.com/topolvm/topolvm/pull/687))

### Fixed

- driver/node: recreate device if owner or device mode is unexpected ([#680](https://github.com/topolvm/topolvm/pull/680))

### Contributors

- @toshipp
- @peng225
- @daichimukai

## [0.18.1] - 2023-03-03

### Added

- add cleanup procedure ([#656](https://github.com/topolvm/topolvm/pull/656))
- Add container-structure-test ([#664](https://github.com/topolvm/topolvm/pull/664))

### Changed

- Clarify lock usage ([#659](https://github.com/topolvm/topolvm/pull/659))

### Contributors

- @peng225
- @bells17
- @toshipp

## [0.18.0] - 2023-02-20

### Added

- add proposal to specify lvcreate options on SC ([#627](https://github.com/topolvm/topolvm/pull/627))
- Add CONTRIBUTING.md ([#631](https://github.com/topolvm/topolvm/pull/631))
- add a note describing how to maintain go version ([#633](https://github.com/topolvm/topolvm/pull/633))
- Add the lvcreate-option-on-storageclass proposal implementation ([#640](https://github.com/topolvm/topolvm/pull/640))
- artifacthub ([#641](https://github.com/topolvm/topolvm/pull/641))

### Changed

- Revert "Drop a PVC finalizer to delete pods(#536)" ([#620](https://github.com/topolvm/topolvm/pull/620))
  - **Note**: The PVC finalizer is not added to the existing PVCs. If the problem explained in [issue #614](https://github.com/topolvm/topolvm/issues/614) happens for those PVCs, you need to resolve it manually.
- build(deps): bump actions/stale from 6 to 7 ([#628](https://github.com/topolvm/topolvm/pull/628))
- Update go directive and use the version for setup-go ([#629](https://github.com/topolvm/topolvm/pull/629))
- Make CI faster again ([#638](https://github.com/topolvm/topolvm/pull/638))
- try to update go 1.19 to fix ci ([#652](https://github.com/topolvm/topolvm/pull/652))

### Fixed

- fix the proposal ([#642](https://github.com/topolvm/topolvm/pull/642))
- fix: set default capacity to thin class capacity when default ([#632](https://github.com/topolvm/topolvm/pull/632))

### Contributors

- @bells17
- @cupnes
- @llamerada-jp
- @peng225
- @suleymanakbas91
- @toshipp

## [0.17.0] - 2023-01-10

### Added

- Added arm64 images ([#600](https://github.com/topolvm/topolvm/pull/600))
- Add ppc64le arch ([#626](https://github.com/topolvm/topolvm/pull/626))

### Changed 

- Support Kubernetes 1.25 ([#610](https://github.com/topolvm/topolvm/pull/610))
- Remove cybozu images ([#621](https://github.com/topolvm/topolvm/pull/621))
- Update api version. ([#630](https://github.com/topolvm/topolvm/pull/630))

### Contributors

- @bells17
- @cupnes
- @toshipp

## [0.16.0] - 2022-12-05

### Caution

This release contains the domain name change([#592](https://github.com/topolvm/topolvm/pull/592)). Before you use this release, **you must choose what to do about this change**. You have two options:
1. Migrate to use `topolvm.io`
2. Continue to use `topolvm.cybozu.com`

If you choose option 1, [this document](https://github.com/topolvm/topolvm/blob/main/docs/proposals/rename-group.md#migrate-from-topolvmcybozucom-to-topolvmio) will help you to migrate your system. Note that this procedure is risky, and you may lose your data. Please back up your data and test the migration procedure before you migrate your production system.

If you choose option 2, you must enable `.Values.useLegacy` flag, otherwise you will lose all of your data.

Check [this document](https://github.com/topolvm/topolvm/blob/main/docs/proposals/rename-group.md) before you upgrade your system to this version.

### Added

- Add health check ([#594](https://github.com/topolvm/topolvm/pull/594))
- add issute template to update supporting kubernetes ([#598](https://github.com/topolvm/topolvm/pull/598))

### Changed

- change reviewer from nbalacha to suleymanakbas91 ([#591](https://github.com/topolvm/topolvm/pull/591))
- add a command to list the relevant PRs in the release procedure. ([#590](https://github.com/topolvm/topolvm/pull/590))
- topolvm.io ([#592](https://github.com/topolvm/topolvm/pull/592))
  - **BREAKING**: Changed the default domain name used in the CRD and plugin name of TopoLVM from `topolvm.cybozu.com` to `topolvm.io`.
- improve issue template ([#602](https://github.com/topolvm/topolvm/pull/602))

### Fixed

- fix: consider spare-gb in free space calculations ([#597](https://github.com/topolvm/topolvm/pull/597))

### Contributors

- @cupnes
- @peng225
- @bells17
- @toshipp
- @nbalacha

## [0.15.3] - 2022-11-04

### Changed

- build(deps): bump actions/stale from 5 to 6 (#571)
- github/workflows: Use output parameter instead of set-output command (#581)
- lvmd: refactor getLVMState() (#584)

### Contributors

- @daichimukai
- @pluser

## [0.15.2] - 2022-10-04

### Added

- Update e2e tests to handle snapshot csi tests (#545)
- e2e: Snapshot-Restore and PVC-PVC cloning features (#546)

### Changed

- Improve the e2e test cases run (#557)
- Use discussions instead of slack. (#565)

### Fixed

- doc: update target name (#554)
- docs: fix incorrect flag description (#556)
- Correct LV JSON major/minor (#561)

### Contributors

- @Yuggupta27
- @bells17
- @nbalacha
- @pluser
- @tasleson
- @toshipp

## [0.15.1] - 2022-08-17

### Fixed

- Make lvm commands independent of the environment (#551) 

## [0.15.0] - 2022-08-16

### Added

- Add ESASHIKA Kaoru as a reviewer (#533)
- e2e tests for thin provisioning feature (#532)
- additional metric for thin pools (#537)

### Changed

- support Kubernetes 1.24 (#529)
- Use lvm JSON output (#501)
- Drop a PVC finalizer to delete pods (#536)

### Doc

- doc: Add design doc for thin-snapshots (#446)
- doc: Add design doc for thin-lv clones (#447)
- Docs: delete description for inline ephemeral volume. (#543)
- fix documentation link (#538)

### Fixed

- webhook: allow PVCs with storageClassName set to "" (#525)
- lvm: disable activation skip (#534)
- fix ci by adding Eventually to wait to start topolvm-controller (#542)
- fix: ResizeLV does not check thinpool overprovisioned size (#540)

### Contributors

- @Yuggupta27
- @tasleson
- @nbalacha
- @usefss

## [0.14.1] - 2022-07-05

### Fixed

- mount: remove new UUID generation (#522)

### Contributors

- @Yuggupta27

## [0.14.0] - 2022-07-04

### Added

- Add support to create PVC-PVC Clones for thin volumes (#498)
- automate adding items to project (#504)
- Update github-actions automatically (#505)
- Add Nithya Balachandran as a reviewer (#506)

### Changed

- Removed: Inline Ephemeral Volume (#494)
  - **BREAKING**: Inline Ephemeral Volume is no longer supported.
- example: wait for topolvm controller mutating webhook to become ready (#500)
- update how to maintain sidecar's RBAC (#508)
- example: retry applying sample pods and pvcs manifest (#509)
- Remove setup-python (#510)
- Remove inline ephemeral volume logic (#519)

### Contributors

- @bells17
- @Yuggupta27

## [0.13.0] - 2022-06-20

### Added

 - Add support for creation of thin-snapshots (#463)

### Contributors

- @Yuggupta27
- @nbalacha

## [0.12.0] - 2022-06-03

### Changed

- Send VG metrics as part of ThinPool metrics (#481)
  - **BREAKING**: `free_bytes` and `size_bytes` metrics report the VG's free space and size respectively even for thinpool.

### Contributors

- @leelavg

## [0.11.1] - 2022-05-09

### Changed

- Modified to use ghcr.io as a container registry (#464)

### Fixed

- Modified to use k8s.io/utils/io.ConsistentRead (#465)
- Set fail-fast option to false (#472)
- Fix github user (#476)
- fix: send correct proto.WatchResponse for thin and thick lvs (#467)
- Fix sudo tests (#471)

### Doc

- nit: correct provisioner typo (#474)

### Contributors

- @bells17
- @Yuggupta27
- @leelavg

## [0.11.0] - 2022-04-14

### Added

- thinp: Implementation of Thin Logical Volumes support (#448)
- Add /readyz endpoint (#457)

### Fixed

- Correct scoring calculation to match codebase (#443)

### Doc

- Design doc for thin logical volumes (#442)
- update thin lv design doc to reflect implemenation (#461)

### Contributors

- @awigen
- @sp98
- @leelavg
- @bells17

## [0.10.6] - 2022-03-03

### Added
- ✨ Add lvcreate-options (#433)

### Contributors
- @lentzi90

## [0.10.5] - 2022-02-04

### Deprecated

- The following features are deprecated. Please see [README](README.md) for details.
  - Ephemeral inline volume
  - Pod security policy

### Changed
- Support Kubernetes v1.23 (#431)

### Added
- add readinessProbe for scheduler (#427)
- Add proposal for lvcreate options (#420)

### Fixed
- Fix flaky (#419)
- Fix flakiness from not configured kube-scheduler. (#430)
- cast Statfs_t.Frsize to int64 for s390x arch (#425)
- fix deploy.md for manual certificate setup. (#428)

### Contributors
- @sp98
- @lentzi90

## [0.10.4] - 2022-01-07

### Added
- Add topolvm-controller CLI flag to skip node finalize (#409)

### Contributors
- @macaptain

## [0.10.3] - 2021-12-01

### Changed
- Support Kubernetes v1.22 (#394)

### Contributors
- @bells17

## [0.10.2] - 2021-11-01

### Added
- Support ReadWriteOncePod feature gate (#345)

### Contributors
- @bells17

## [0.10.1] - 2021-10-18

### Fixed
- Restart csi-registrar if registration is failed. (#374)
- Fix typo for KubeSchedulerConfiguration (#367)

### Added
- Add logo (#376)

### Contributors
- @superbrothers

## [0.10.0] - 2021-09-13

### Changed
- Change license to Apache License Version 2.0. (#360)

### Fixed
- Bugs: Fix nsenter -a args to nsenter -m -u -i -n -p -t 1 (#364)

### Contributors
- @attlee-wang

## [0.9.2] - 2021-09-07

### Added
- Add btrfs-progs pkg to use btrfs commands. (#357)

### Changed
- Replace resizefs.go with mount-utils (#356)

## [0.9.1] - 2021-09-06

### Misc
- Fix YAML example in deploy/README.md (#352)
- Fix typo in a comment (#355)

### Contributors
- @nnstt1
- @nonylene

## [0.9.0] - 2021-08-10
### Added
- Add topolvm helm charts (#302)
  - **BREAKING**: Some resource names or labels are changed. You need to delete previous manifests, then install helm chart.
- Add recommended labels (#320)
- Add Storage Capacity Tracking mode (#315)

### Changed
- support k8s 1.21 (#299)
- update conformed csi version (#322)
- Add error message (#314)
- Remove CSI Attacher sidecar (#319)
  - **BREAKING**: You need to recreate the CSIDriver resource because CSI Attacher sidecar was removed from topolvm.

### Removed
- Remove kustomize manifests. (#336)

### Fixed
- add document to run test using minikube for lvmd daemonset (#318)
- Fix lint error (#321)
- Migrate E2E manifests from kustomize to Helm (#325)

### Contributors
- @toelke
- @bells17
- @d-kuro

# [0.8.3] - 2021-05-11

### Added
- Modify to use control-plane label and taint (#295)

### Changed
- Regenerate programs and manifests using kubebuilder v3 (#298)
- Use Ubuntu 18.04 as the base image for TopoLVM container (#306)

### Fixed
- Stabilize Docker build (#304)
- driver: fix overwriting error in NodePublishVolume (#305)
- change urls of placeholder images (#307)
- Fix duplicating finalizer entry (#311)

### Contributors
- @bells17

## [0.8.2] - 2021-04-07

### Added
- Add topolvm_volumegroup_size_bytes metric (#290)

### Fixed
- Add tests for DaemonSet lvmd (#285)
- fix PodSecurityPolicy for Daemonset of lvmd (#291)

### Contributors
- @bells17

## [0.8.1] - 2021-03-22

### Fixed
- Make event handler be invoked on create event (#279)
- Avoid unnecessary readlink (#276)

### Changed
- Update golang to 1.16 (#281)

## [0.8.0] - 2021-03-05

### Fixed
- update kubebuilder options to accept dry-run (#251)

### Added
- Support mount option (#260)

### Changed
- Support k8s 1.20 (#259)
  - **BREAKING**: Drop support for `admissionregistration.k8s.io/v1beta1`
  - The default port for the webhook was changed to 9443.
  - The options for logger were changed according to use zap.
- Update the CSI spec to v1.3.0 (#256)
- Rename master branch to main (#255)
- Purge official sidecar images from e2e (#249)
- Add a minimum image (#236)
  - **BREAKING**: As of v0.7.0, `topolvm/topolvm` image does not contain sidecar binaries. if you wish to use images containing sidecar binaries, use `topolvm/topolvm-with-sidecar` instead.
- force stopping kubelet to unmount lv volumes (#245)
- add a note for example to suggest tagged version (#266)
- add a note about host's kernel to supported environments (#262)
- fix to use topolvm-with-sidecar for example test (#263)

### Contributors
- @bells17

## [0.7.0] - 2021-01-18

### Added
- Support striped LV (#229)

### Changed
- Update protoc-gen-go-grpc and regenerate stubs (#213)
- Build hypertopolvm inside docker (#215)
- modify manifest for deployment-scheduler (#221)
- Add CSI ephemeral volumes limitation (#224)
- fix defaultDivisor not work (#225)
- Improve stale LV deletion method (#230)
- Use MayRunAs instead of MustRunAs to avoid recursive chown (#233)
- Update cert-manager API ver to v1 (#234)
- Add support for k8s 1.19. (#237)
    - k8s 1.19 only supports `kubescheduler.config.k8s.io/v1beta1`
- Go 1.15 and Ubuntu 20.04 (#239)

### Removed
- drop btrfs support (#240)
- No longer use the vendor function (#237)

### Experimental

- Support running lvmd in a container (#208)

### Contributors
- @bells17
- @onokatio
- @UZER0

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

[Unreleased]: https://github.com/topolvm/topolvm/compare/v0.22.0...HEAD
[0.22.0]: https://github.com/topolvm/topolvm/compare/v0.21.0...v0.22.0
[0.21.0]: https://github.com/topolvm/topolvm/compare/v0.20.0...v0.21.0
[0.20.0]: https://github.com/topolvm/topolvm/compare/v0.19.1...v0.20.0
[0.19.1]: https://github.com/topolvm/topolvm/compare/v0.19.0...v0.19.1
[0.19.0]: https://github.com/topolvm/topolvm/compare/v0.18.2...v0.19.0
[0.18.2]: https://github.com/topolvm/topolvm/compare/v0.18.1...v0.18.2
[0.18.1]: https://github.com/topolvm/topolvm/compare/v0.18.0...v0.18.1
[0.18.0]: https://github.com/topolvm/topolvm/compare/v0.17.0...v0.18.0
[0.17.0]: https://github.com/topolvm/topolvm/compare/v0.16.0...v0.17.0
[0.16.0]: https://github.com/topolvm/topolvm/compare/v0.15.3...v0.16.0
[0.15.3]: https://github.com/topolvm/topolvm/compare/v0.15.2...v0.15.3
[0.15.2]: https://github.com/topolvm/topolvm/compare/v0.15.1...v0.15.2
[0.15.1]: https://github.com/topolvm/topolvm/compare/v0.15.0...v0.15.1
[0.15.0]: https://github.com/topolvm/topolvm/compare/v0.14.1...v0.15.0
[0.14.1]: https://github.com/topolvm/topolvm/compare/v0.14.0...v0.14.1
[0.14.0]: https://github.com/topolvm/topolvm/compare/v0.13.0...v0.14.0
[0.13.0]: https://github.com/topolvm/topolvm/compare/v0.12.0...v0.13.0
[0.12.0]: https://github.com/topolvm/topolvm/compare/v0.11.1...v0.12.0
[0.11.1]: https://github.com/topolvm/topolvm/compare/v0.11.0...v0.11.1
[0.11.0]: https://github.com/topolvm/topolvm/compare/v0.10.6...v0.11.0
[0.10.6]: https://github.com/topolvm/topolvm/compare/v0.10.5...v0.10.6
[0.10.5]: https://github.com/topolvm/topolvm/compare/v0.10.4...v0.10.5
[0.10.4]: https://github.com/topolvm/topolvm/compare/v0.10.3...v0.10.4
[0.10.3]: https://github.com/topolvm/topolvm/compare/v0.10.2...v0.10.3
[0.10.2]: https://github.com/topolvm/topolvm/compare/v0.10.1...v0.10.2
[0.10.1]: https://github.com/topolvm/topolvm/compare/v0.10.0...v0.10.1
[0.10.0]: https://github.com/topolvm/topolvm/compare/v0.9.2...v0.10.0
[0.9.2]: https://github.com/topolvm/topolvm/compare/v0.9.1...v0.9.2
[0.9.1]: https://github.com/topolvm/topolvm/compare/v0.9.0...v0.9.1
[0.9.0]: https://github.com/topolvm/topolvm/compare/v0.8.3...v0.9.0
[0.8.3]: https://github.com/topolvm/topolvm/compare/v0.8.2...v0.8.3
[0.8.2]: https://github.com/topolvm/topolvm/compare/v0.8.1...v0.8.2
[0.8.1]: https://github.com/topolvm/topolvm/compare/v0.8.0...v0.8.1
[0.8.0]: https://github.com/topolvm/topolvm/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/topolvm/topolvm/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/topolvm/topolvm/compare/v0.5.3...v0.6.0
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
