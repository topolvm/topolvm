[![GitHub release](https://img.shields.io/github/release/cybozu-go/well.svg?maxAge=60)][releases]
[![GoDoc](https://godoc.org/github.com/cybozu-go/well?status.svg)][godoc]
[![CircleCI](https://circleci.com/gh/cybozu-go/well.svg?style=svg)](https://circleci.com/gh/cybozu-go/well)
[![Go Report Card](https://goreportcard.com/badge/github.com/cybozu-go/well)](https://goreportcard.com/report/github.com/cybozu-go/well)

Go Command Framework
====================

This is a framework to create well-behaving commands.

Features
--------

* [Context](https://golang.org/pkg/context/)-based goroutine management.
* Signal handlers.
* Graceful stop/restart for any kind of network servers.
* Logging options.
* Enhanced [http.Server](https://golang.org/pkg/net/http/#Server).
* Ultra fast UUID-like ID generator.
* Activity tracking.
* Support for [systemd socket activation](http://0pointer.de/blog/projects/socket-activation.html).
* Support for [github.com/spf13/cobra][cobra].

Requirements
------------

Go 1.11 or later, though the code is expected to work with Go 1.7.

Specifications
--------------

Commands using this framework implement these external specifications:

### Command-line options

* `-logfile FILE`

    Output logs to FILE instead of standard error.

* `-loglevel LEVEL`

    Change logging threshold to LEVEL.  Default is `info`.  
    LEVEL is one of `critical`, `error`, `warning`, `info`, or `debug`.

* `-logformat FORMAT`

    Change log formatter.  Default is `plain`.  
    FORMAT is one of `plain`, `logfmt`, or `json`.

### Signal Handlers

* `SIGUSR1`

    If `-logfile` is specified, this signal make the program reopen
    the log file to cooperate with an external log rotation program.

    On Windows, this is not implemented.

* `SIGINT` and `SIGTERM`

    These signals cancel the context of the global environment,
    and hence goroutines registered with the environment.  Usually
    this will result in graceful stop of network servers, if any.

    On Windows, only `SIGINT` is handled.

* `SIGHUP`

    This signal is used to restart network servers gracefully.
    Internally, the main (master) process restarts its child process.
    The PID of the master process thus will not change.

    There is one limitation: the location of log file cannot be changed
    by graceful restart.  To change log file location, the server need
    to be (gracefully) stopped and started.

    On Windows, this is not implemented.

* `SIGPIPE`

    The framework changes [the way Go handles SIGPIPE slightly](https://golang.org/pkg/os/signal/#hdr-SIGPIPE).
    If a program using this framework receives SIGPIPE when writing to stdout or stderr, the program exits with status code 2.
    See [#15](https://github.com/cybozu-go/well/issues/15) for details.

### Environment variables

* `REQUEST_ID_HEADER`

    The value of this variable is used as HTTP header name.
    The HTTP header is used to track activities across services.
    The default header name is "X-Cybozu-Request-ID".

* `CYBOZU_LISTEN_FDS`

    This is used internally for graceful restart.

Usage
-----

Read [Tutorial][wiki], [the design notes](DESIGN.md) and [godoc][].

A [wiki page for cobra users](https://github.com/cybozu-go/well/wiki/Use-with-spf13-cobra) is also available.

Real world examples
-------------------

* [`github.com/cybozu-go/coil`](https://github.com/cybozu-go/coil) uses well with [cobra][].
* [`github.com/cybozu-go/aptutil`](https://github.com/cybozu-go/aptutil)
* [`github.com/cybozu-go/goma`](https://github.com/cybozu-go/goma)
* [`github.com/cybozu-go/transocks`](https://github.com/cybozu-go/transocks)
* [`github.com/cybozu-go/usocksd`](https://github.com/cybozu-go/usocksd)

Pull requests are welcome to add your project to this list!

License
-------

[MIT][]

[releases]: https://github.com/cybozu-go/well/releases
[godoc]: https://godoc.org/github.com/cybozu-go/well
[wiki]: https://github.com/cybozu-go/well/wiki/Tutorial
[cobra]: https://github.com/spf13/cobra
[MIT]: https://opensource.org/licenses/MIT
