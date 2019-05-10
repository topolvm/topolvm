# Change Log

All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]

## [1.2.0] - 2018-09-14
### Added
- Opt in to [Go modules](https://github.com/golang/go/wiki/Modules).

## [1.1.0] - 2016-09-11
### Added
- `CipherSuiteString` returns string for tls.TLS_* constants.
- `TLSVersionString` returns string for tls.Version* constants.

## [1.0.1] - 2016-08-28
### Added
- `IsNetworkUnreachable`, `IsConnectionRefused`, `IsNoRouteToHost` functions to identify network errors.

[Unreleased]: https://github.com/cybozu-go/log/compare/v1.2.0...HEAD
[1.2.0]: https://github.com/cybozu-go/log/compare/v1.1.0...v1.2.0
[1.1.0]: https://github.com/cybozu-go/log/compare/v1.0.1...v1.1.0
[1.0.1]: https://github.com/cybozu-go/log/compare/v1.0.0...v1.0.1
