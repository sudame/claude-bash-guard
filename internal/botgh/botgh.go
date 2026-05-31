// Package botgh runs gh commands under a GitHub App (bot) identity by minting a
// short-lived installation access token and passing it to gh via GH_TOKEN.
package botgh

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/sudame/claude-bash-guard/internal/config"
	"github.com/sudame/claude-bash-guard/internal/githubapp"
	"github.com/sudame/claude-bash-guard/internal/keychain"
)

const (
	defaultKeychainService = "claude-botpr"
	defaultKeychainAccount = "pem"
)

// Token mints a short-lived GitHub App installation access token from the
// configured App credentials and the private key stored in the macOS keychain.
func Token(ctx context.Context) (string, error) {
	b := config.Load().Botpr
	if !b.Configured() {
		return "", fmt.Errorf("botpr is not configured; set botpr.app_id and botpr.installation_id in the config file")
	}

	service := b.KeychainService
	if service == "" {
		service = defaultKeychainService
	}
	account := b.KeychainAccount
	if account == "" {
		account = defaultKeychainAccount
	}

	pemKey, err := keychain.Find(service, account)
	if err != nil {
		return "", err
	}

	jwt, err := githubapp.MintJWT(b.AppID, []byte(pemKey), time.Now())
	if err != nil {
		return "", err
	}

	return githubapp.InstallationToken(ctx, http.DefaultClient, jwt, b.InstallationID)
}

// ExecGh runs `gh <args...>` with GH_TOKEN set to token, forwarding stdio. It
// returns gh's exit code (0 on success); err is non-nil only when gh could not
// be launched. The child gh process is not a Claude Code Bash tool call, so the
// bash-guard hook does not intercept it.
func ExecGh(token string, args []string) (int, error) {
	cmd := exec.Command("gh", args...)
	cmd.Env = append(os.Environ(), "GH_TOKEN="+token)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	return 1, fmt.Errorf("running gh: %w", err)
}
