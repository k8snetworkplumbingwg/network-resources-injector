package webhook

import (
	"crypto/tls"
	"fmt"
	"strconv"
	"strings"

	cliflag "k8s.io/component-base/cli/flag"
)

// CipherNamesToIDs converts a slice of cipher suite names to the corresponding
// crypto/tls cipher suite IDs using Kubernetes component-base flag helpers.
// Accepted values are the same as cliflag.TLSCipherPossibleValues().
func CipherNamesToIDs(names []string) ([]uint16, error) {
	trimmedNames := make([]string, 0, len(names))
	for _, name := range names {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			return nil, fmt.Errorf("empty TLS cipher suite name")
		}
		trimmedNames = append(trimmedNames, trimmedName)
	}

	ids, err := cliflag.TLSCipherSuites(trimmedNames)
	if err != nil {
		return nil, err
	}

	insecureCipherIDs := map[uint16]struct{}{}
	for _, id := range cliflag.InsecureTLSCiphers() {
		insecureCipherIDs[id] = struct{}{}
	}

	for i, id := range ids {
		if _, found := insecureCipherIDs[id]; found {
			return nil, fmt.Errorf("TLS cipher suite %q is insecure and not allowed", trimmedNames[i])
		}
	}

	return ids, nil
}

// ParseTLSCipherSuites converts a comma-separated list of TLS cipher suites to
// crypto/tls cipher suite IDs. Empty input returns nil to use Go runtime
// defaults.
func ParseTLSCipherSuites(cipherSuites string) ([]uint16, error) {
	trimmed := strings.TrimSpace(cipherSuites)
	if trimmed == "" {
		return nil, nil
	}
	return CipherNamesToIDs(strings.Split(trimmed, ","))
}

// ParseCurvePreferences parses a comma-separated list of numeric TLS CurveID
// values (as defined in crypto/tls) and returns them as []tls.CurveID.
// For example "29,23,24" selects X25519, CurveP256, and CurveP384.
// The supported values depend on the Go version used.
// See https://pkg.go.dev/crypto/tls#CurveID for values supported for each Go version.
// Returns nil if the input is empty.
func ParseCurvePreferences(curvePreferences string) ([]tls.CurveID, error) {
	trimmed := strings.TrimSpace(curvePreferences)
	if trimmed == "" {
		return nil, nil
	}

	parts := strings.Split(trimmed, ",")
	curves := make([]tls.CurveID, 0, len(parts))
	for _, part := range parts {
		s := strings.TrimSpace(part)
		if s == "" {
			return nil, fmt.Errorf("empty TLS CurveID value")
		}
		v, err := strconv.ParseUint(s, 10, 16)
		if err != nil {
			return nil, fmt.Errorf("invalid TLS CurveID value %q: %w", s, err)
		}
		curves = append(curves, tls.CurveID(v))
	}

	return curves, nil
}

// TLSVersionToGo converts a TLS version string (for example "VersionTLS12") to
// the corresponding crypto/tls version constant. Empty input defaults to
// VersionTLS12.
func TLSVersionToGo(version string) (uint16, error) {
	trimmedVersion := strings.TrimSpace(version)
	if trimmedVersion == "" {
		return tls.VersionTLS12, nil
	}
	goVersion, err := cliflag.TLSVersion(trimmedVersion)
	if err != nil {
		return 0, err
	}
	if goVersion < tls.VersionTLS12 {
		return 0, fmt.Errorf("minimum TLS version must be VersionTLS12 or higher, got %q", trimmedVersion)
	}
	return goVersion, nil
}
