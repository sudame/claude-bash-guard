// Package keychain reads secrets from the macOS keychain via the `security` CLI.
// macOS only for now; extend with build tags when another platform is needed.
package keychain

import (
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
)

// Find returns the generic-password data stored under the given service and
// account.
//
// `security ... -w` returns the value hex-encoded when it is not a plain
// single-line string (a multi-line PEM key triggers this), so an all-hex
// result is decoded back to its original bytes. A PEM key is never itself all
// hex, so this is unambiguous. The trailing newline that `security` appends is
// trimmed.
func Find(service, account string) (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-s", service, "-a", account, "-w").Output()
	if err != nil {
		return "", fmt.Errorf("keychain lookup failed for service=%q account=%q: %w", service, account, err)
	}
	s := strings.TrimRight(string(out), "\n")
	if decoded, ok := decodeHex(s); ok {
		return decoded, nil
	}
	return s, nil
}

// decodeHex reports whether s is an even-length all-hex string and, if so,
// returns its decoded bytes.
func decodeHex(s string) (string, bool) {
	if len(s) == 0 || len(s)%2 != 0 {
		return "", false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F':
		default:
			return "", false
		}
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return "", false
	}
	return string(b), true
}
