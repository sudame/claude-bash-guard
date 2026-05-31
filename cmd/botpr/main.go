// Command botpr creates a pull request authored by a GitHub App (bot) identity.
//
// It mints an installation access token from a GitHub App private key stored in
// the macOS keychain, then runs `gh pr create` with that token so the resulting
// PR is authored by the app[bot]. All arguments are passed straight through to
// `gh pr create`. Commits keep their local git authorship; only the PR author
// becomes the bot, which is what lets you review and approve it yourself.
package main

import (
	"context"
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

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "botpr: "+err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	b := config.Load().Botpr
	if !b.Configured() {
		return fmt.Errorf("botpr is not configured; set botpr.app_id and botpr.installation_id in the config file")
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
		return err
	}

	jwt, err := githubapp.MintJWT(b.AppID, []byte(pemKey), time.Now())
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	token, err := githubapp.InstallationToken(ctx, http.DefaultClient, jwt, b.InstallationID)
	if err != nil {
		return err
	}

	return execGhPrCreate(args, token)
}

// execGhPrCreate runs `gh pr create <args...>` with GH_TOKEN set to the bot
// token so the PR is authored by the app[bot]. The child gh process is not a
// Claude Code Bash tool call, so the bash-guard hook does not intercept it.
func execGhPrCreate(args []string, token string) error {
	cmd := exec.Command("gh", append([]string{"pr", "create"}, args...)...)
	cmd.Env = append(os.Environ(), "GH_TOKEN="+token)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			os.Exit(exit.ExitCode())
		}
		return fmt.Errorf("running gh pr create: %w", err)
	}
	return nil
}
