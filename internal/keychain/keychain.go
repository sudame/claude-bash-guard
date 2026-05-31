// Package keychain reads secrets from the macOS keychain via the `security` CLI.
// macOS only for now; extend with build tags when another platform is needed.
package keychain

import (
	"fmt"
	"os/exec"
	"strings"
)

// Find returns the generic-password data stored under the given service and
// account. The trailing newline that `security -w` appends is removed; PEM
// content is otherwise returned verbatim.
func Find(service, account string) (string, error) {
	out, err := exec.Command("security", "find-generic-password", "-s", service, "-a", account, "-w").Output()
	if err != nil {
		return "", fmt.Errorf("keychain lookup failed for service=%q account=%q: %w", service, account, err)
	}
	return strings.TrimRight(string(out), "\n"), nil
}
