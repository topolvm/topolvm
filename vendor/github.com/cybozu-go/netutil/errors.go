package netutil

import "strings"

// network errors in golang are difficult to distinguish.
// src/syscall/zerrors_linux_amd64.go has these definitions:
//     var errors = [...]string{
//         ...
//         101: "network is unreachable",
//         ...
//         111: "connection refused",
//         112: "host is down",
//         113: "no route to host",
//         ...
//     }
//
// The same strings appear in other operating systems / architectures.
//
// As these strings are used to stringify syscall.Errno, we can identify
// class of network errors by using err.Error() string.

// IsNetworkUnreachable returns true if err indicates ENETUNREACH errno.
func IsNetworkUnreachable(err error) bool {
	return strings.Contains(err.Error(), "network is unreachable")
}

// IsConnectionRefused returns true if err indicates ECONNREFUSED errno.
func IsConnectionRefused(err error) bool {
	return strings.Contains(err.Error(), "connection refused")
}

// IsNoRouteToHost returns true if err indicates EHOSTUNREACH errno.
func IsNoRouteToHost(err error) bool {
	return strings.Contains(err.Error(), "no route to host")
}
