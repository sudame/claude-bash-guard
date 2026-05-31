// Package config loads the shared claude-bash-guard configuration file used by
// both the bash-guard hook and the botpr PR helper.
package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the on-disk configuration shared by all binaries in this repo.
type Config struct {
	// Account is the GitHub account the AI must use for `gh pr create` in the
	// legacy account-switch mode. Empty disables that check. Ignored when Botpr
	// is configured.
	Account string `yaml:"account"`
	// ExcludePaths lists working-directory prefixes where the gh pr create check is skipped.
	ExcludePaths []string `yaml:"exclude_paths"`
	// DisabledRules lists rule IDs that are turned off globally.
	DisabledRules []string `yaml:"disabled_rules"`
	// Botpr configures the GitHub App used to author PRs as a bot.
	Botpr Botpr `yaml:"botpr"`
}

// Botpr holds the GitHub App credentials botpr uses to mint installation tokens.
type Botpr struct {
	// AppID is the numeric GitHub App ID.
	AppID int64 `yaml:"app_id"`
	// InstallationID is the App installation ID on the target account/org.
	InstallationID int64 `yaml:"installation_id"`
	// KeychainService/KeychainAccount locate the private key in the macOS keychain.
	KeychainService string `yaml:"keychain_service"`
	KeychainAccount string `yaml:"keychain_account"`
}

// Configured reports whether enough is set to mint installation tokens.
func (b Botpr) Configured() bool {
	return b.AppID != 0 && b.InstallationID != 0
}

// Path resolves the configuration file location.
func Path() string {
	if p := os.Getenv("CLAUDE_BASH_GUARD_CONFIG"); p != "" {
		return p
	}
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "claude-bash-guard.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "claude-bash-guard.yaml")
}

// Load reads and parses the config file, returning a zero Config on any error.
func Load() Config {
	path := Path()
	if path == "" {
		return Config{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}
	}
	return c
}

// Excludes reports whether cwd falls under any configured exclude prefix.
func (c Config) Excludes(cwd string) bool {
	for _, p := range c.ExcludePaths {
		if p != "" && strings.HasPrefix(cwd, p) {
			return true
		}
	}
	return false
}

// Disabled reports whether the given rule ID is globally disabled.
func (c Config) Disabled(rule string) bool {
	return slices.Contains(c.DisabledRules, rule)
}
