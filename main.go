package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type hookInput struct {
	Cwd       string `json:"cwd"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type config struct {
	// Account is the GitHub account the AI must use for `gh pr create`.
	// Empty disables the check entirely.
	Account string `yaml:"account"`
	// ExcludePaths lists working-directory prefixes where the check is skipped.
	ExcludePaths []string `yaml:"exclude_paths"`
	// DisabledRules lists rule IDs that are turned off globally. See the rule* constants.
	DisabledRules []string `yaml:"disabled_rules"`
}

// Rule IDs usable in config.DisabledRules.
const (
	ruleChaining     = "chaining"
	ruleGitDashC     = "git_dash_c"
	ruleCd           = "cd"
	ruleGhApiWrite   = "gh_api_write"
	ruleAwsNoProfile = "aws_no_profile"
	ruleGhPrCreate   = "gh_pr_create"
)

func configPath() string {
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

func loadConfig() config {
	path := configPath()
	if path == "" {
		return config{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return config{}
	}
	var c config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return config{}
	}
	return c
}

func (c config) excludes(cwd string) bool {
	for _, p := range c.ExcludePaths {
		if p != "" && strings.HasPrefix(cwd, p) {
			return true
		}
	}
	return false
}

func (c config) disabled(rule string) bool {
	for _, r := range c.DisabledRules {
		if r == rule {
			return true
		}
	}
	return false
}

type askOutput struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

var (
	reQuotedSingle = regexp.MustCompile(`'[^']*'`)
	reQuotedDouble = regexp.MustCompile(`"[^"]*"`)
	reChaining     = regexp.MustCompile(`&&|;`)
	reGitDashC     = regexp.MustCompile(`^\s*git\s+-C\b`)
	reCd           = regexp.MustCompile(`^\s*cd\b`)
	reGhApi        = regexp.MustCompile(`^\s*gh\s+api\b`)
	reWriteMethod  = regexp.MustCompile(`(-X|--method)\s`)
	reReviewReply  = regexp.MustCompile(`/comments/[^/\s]+/replies\b`)
	reAws          = regexp.MustCompile(`^\s*aws\s`)
	reProfileFlag  = regexp.MustCompile(`--profile\b`)
	reGhPrCreate   = regexp.MustCompile(`^\s*gh\s+pr\s+create\b`)
	reActiveAcct   = regexp.MustCompile(`account\s+(\S+)`)
)

// activeAccountFunc resolves the currently active gh account login.
// It is a package variable so tests can stub it.
var activeAccountFunc = ghActiveAccount

func ghActiveAccount() string {
	out, err := exec.Command("gh", "auth", "status", "--active").CombinedOutput()
	if err != nil {
		return ""
	}
	m := reActiveAcct.FindSubmatch(out)
	if m == nil {
		return ""
	}
	return string(m[1])
}

type decision int

const (
	allow decision = iota
	blockChaining
	blockGitDashC
	blockCd
	askGhApiWrite
	blockAwsNoProfile
	blockGhPrCreateNotBot
)

func (d decision) isBlock() bool {
	return d == blockChaining || d == blockGitDashC || d == blockCd || d == blockAwsNoProfile || d == blockGhPrCreateNotBot
}

func (d decision) isAsk() bool {
	return d == askGhApiWrite
}

func (d decision) message(account string) string {
	switch d {
	case blockChaining:
		return "コマンド連結(&&, ;)は禁止。1コマンドずつ実行してください。"
	case blockGitDashC:
		return "git -C は禁止。aicd で移動してから実行してください。"
	case blockCd:
		return "cd は禁止。代わりに aicd を使ってください。"
	case askGhApiWrite:
		return "gh api の書き込みメソッド(-X/--method)は都度確認"
	case blockAwsNoProfile:
		return "aws は --profile を指定してください。"
	case blockGhPrCreateNotBot:
		return "gh pr create は " + account + " で行ってください。`gh auth switch --user " + account + "` で切り替えてから実行してください。"
	}
	return ""
}

func evaluate(cmd, cwd string, cfg config) decision {
	stripped := reQuotedDouble.ReplaceAllString(reQuotedSingle.ReplaceAllString(cmd, ""), "")
	if !cfg.disabled(ruleChaining) && reChaining.MatchString(stripped) {
		return blockChaining
	}
	if !cfg.disabled(ruleGitDashC) && reGitDashC.MatchString(cmd) {
		return blockGitDashC
	}
	if !cfg.disabled(ruleCd) && reCd.MatchString(cmd) {
		return blockCd
	}
	if !cfg.disabled(ruleGhApiWrite) && reGhApi.MatchString(cmd) && reWriteMethod.MatchString(cmd) {
		if reReviewReply.MatchString(cmd) {
			return allow
		}
		return askGhApiWrite
	}
	if !cfg.disabled(ruleAwsNoProfile) && reAws.MatchString(cmd) && !reProfileFlag.MatchString(cmd) {
		return blockAwsNoProfile
	}
	if !cfg.disabled(ruleGhPrCreate) && reGhPrCreate.MatchString(cmd) && cfg.Account != "" && !cfg.excludes(cwd) {
		if activeAccountFunc() != cfg.Account {
			return blockGhPrCreateNotBot
		}
	}
	return allow
}

func main() {
	raw, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Exit(0)
	}

	var in hookInput
	if err := json.Unmarshal(raw, &in); err != nil {
		os.Exit(0)
	}

	cmd := in.ToolInput.Command
	if cmd == "" {
		os.Exit(0)
	}

	cfg := loadConfig()
	d := evaluate(cmd, in.Cwd, cfg)
	switch {
	case d.isAsk():
		out := askOutput{}
		out.HookSpecificOutput.HookEventName = "PreToolUse"
		out.HookSpecificOutput.PermissionDecision = "ask"
		out.HookSpecificOutput.PermissionDecisionReason = d.message(cfg.Account)
		_ = json.NewEncoder(os.Stdout).Encode(out)
		os.Exit(0)
	case d.isBlock():
		fmt.Fprintln(os.Stderr, "BLOCKED: "+d.message(cfg.Account))
		os.Exit(2)
	}
	os.Exit(0)
}
