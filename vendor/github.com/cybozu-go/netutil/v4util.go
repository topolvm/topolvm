package netutil

import (
	"encoding/binary"
	"errors"
	"net"
)

var (
	// ErrIPv6 is an error that a function does not support IPv6.
	ErrIPv6 = errors.New("only IPv4 address is supported")
)

// IP4ToInt returns uint32 value for an IPv4 address.
// If ip is not an IPv4 address, this returns 0.
func IP4ToInt(ip net.IP) uint32 {
	ip = ip.To4()
	if ip == nil {
		return 0
	}
	return binary.BigEndian.Uint32(ip)
}

// IntToIP4 does the reverse of IP4ToInt.
func IntToIP4(n uint32) net.IP {
	ip := make([]byte, 4)
	binary.BigEndian.PutUint32(ip, n)
	return ip
}

// HostsFunc returns a function to generate all IP addresses in a
// network.  The network address and the broadcast address of the network
// will be excluded.
//
// The returned function will finally generate nil to tell the end.
//
// The network must be an IPv4 network.
func HostsFunc(n *net.IPNet) (func() net.IP, error) {
	if n.IP.To4() == nil {
		return nil, ErrIPv6
	}

	ones, bits := n.Mask.Size()
	count := (1 << uint(bits-ones)) - 2
	current := IP4ToInt(n.IP) + 1
	return func() net.IP {
		if count <= 0 {
			return nil
		}
		ip := IntToIP4(current)
		current++
		count--
		return ip
	}, nil
}
