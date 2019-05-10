/*
Package well provides a framework that helps implementation of
commands having these features:

Better logging:

By using github.com/cybozu-go/log package, logs can be structured
in JSON or logfmt format.  HTTP servers log accesses automatically.

Graceful exit:

The framework provides functions to manage goroutines and
network server implementation that can be shutdown gracefully.

Signal handlers:

The framework installs SIGINT/SIGTERM signal handlers for
graceful exit, and SIGUSR1 signal handler to reopen log files.

Environment

Environment is the heart of the framework.  It provides a base
context.Context that will be canceled before program stops, and
methods to manage goroutines.

To use the framework easily, the framework provides an instance of
Environment as the default, and functions to work with it.
*/
package well
