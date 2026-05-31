package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"

	"github.com/sudame/claude-bash-guard/internal/config"
)

type hookInput struct {
	Cwd       string `json:"cwd"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

// Rule IDs usable in config.Config.DisabledRules.
const (
	ruleChaining     = "chaining"
	ruleGitDashC     = "git_dash_c"
	ruleCd           = "cd"
	ruleGhApiWrite   = "gh_api_write"
	ruleAwsNoProfile = "aws_no_profile"
	ruleGhPrCreate   = "gh_pr_create"
)

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
	blockGhPrCreateUseBotpr
)

func (d decision) isBlock() bool {
	switch d {
	case blockChaining, blockGitDashC, blockCd, blockAwsNoProfile, blockGhPrCreateNotBot, blockGhPrCreateUseBotpr:
		return true
	}
	return false
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
	case blockGhPrCreateUseBotpr:
		return "gh pr create は禁止。bot 名義で PR を作るには botpr を使ってください（引数はそのまま gh pr create に渡されます）。"
	}
	return ""
}

func evaluate(cmd, cwd string, cfg config.Config) decision {
	stripped := reQuotedDouble.ReplaceAllString(reQuotedSingle.ReplaceAllString(cmd, ""), "")
	if !cfg.Disabled(ruleChaining) && reChaining.MatchString(stripped) {
		return blockChaining
	}
	if !cfg.Disabled(ruleGitDashC) && reGitDashC.MatchString(cmd) {
		return blockGitDashC
	}
	if !cfg.Disabled(ruleCd) && reCd.MatchString(cmd) {
		return blockCd
	}
	if !cfg.Disabled(ruleGhApiWrite) && reGhApi.MatchString(cmd) && reWriteMethod.MatchString(cmd) {
		if reReviewReply.MatchString(cmd) {
			return allow
		}
		return askGhApiWrite
	}
	if !cfg.Disabled(ruleAwsNoProfile) && reAws.MatchString(cmd) && !reProfileFlag.MatchString(cmd) {
		return blockAwsNoProfile
	}
	if !cfg.Disabled(ruleGhPrCreate) && reGhPrCreate.MatchString(cmd) && !cfg.Excludes(cwd) {
		if cfg.Botpr.Configured() {
			return blockGhPrCreateUseBotpr
		}
		if cfg.Account != "" && activeAccountFunc() != cfg.Account {
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

	cfg := config.Load()
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
