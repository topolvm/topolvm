package capacitytemplate

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	capacityNamePrefix = "topolvm-template-"
	capacityHashLength = 16
	maxDNSSubdomainLen = 253
)

// CapacityName returns the deterministic CSIStorageCapacity name for an
// identity. NUL-delimited hashing avoids ambiguous concatenations, while a
// readable namespace/name prefix makes the object convenient to inspect.
func CapacityName(namespace, nodeGroupName, storageClass, deviceClass string) string {
	h := sha256.New()
	for _, part := range []string{namespace, nodeGroupName, storageClass, deviceClass} {
		// NUL cannot occur in any Kubernetes name or parameter used here.
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	hash := hex.EncodeToString(h.Sum(nil))[:capacityHashLength]

	readable := dnsReadable(namespace + "-" + nodeGroupName)
	if readable == "" {
		readable = "nodegroup"
	}
	maxReadable := maxDNSSubdomainLen - len(capacityNamePrefix) - 1 - len(hash)
	if len(readable) > maxReadable {
		readable = strings.Trim(readable[:maxReadable], "-.")
	}
	if readable == "" {
		readable = "nodegroup"
	}
	return capacityNamePrefix + readable + "-" + hash
}

func dnsReadable(value string) string {
	value = strings.ToLower(value)
	var b strings.Builder
	b.Grow(len(value))
	separator := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			separator = false
			continue
		}
		// Kubernetes names are ASCII, but treating every other rune as a
		// separator keeps this helper safe for direct unit tests too.
		if !separator && b.Len() != 0 {
			b.WriteByte('-')
		}
		separator = true
	}
	return strings.Trim(b.String(), "-")
}
