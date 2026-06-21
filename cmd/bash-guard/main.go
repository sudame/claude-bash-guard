package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
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
)

type decision int

const (
	allow decision = iota
	blockChaining
	blockGitDashC
	blockCd
	askGhApiWrite
	blockAwsNoProfile
)

func (d decision) isBlock() bool {
	switch d {
	case blockChaining, blockGitDashC, blockCd, blockAwsNoProfile:
		return true
	}
	return false
}

func (d decision) isAsk() bool {
	return d == askGhApiWrite
}

func (d decision) message() string {
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
	}
	return ""
}

func evaluate(cmd string, cfg config.Config) decision {
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
	d := evaluate(cmd, cfg)
	switch {
	case d.isAsk():
		out := askOutput{}
		out.HookSpecificOutput.HookEventName = "PreToolUse"
		out.HookSpecificOutput.PermissionDecision = "ask"
		out.HookSpecificOutput.PermissionDecisionReason = d.message()
		_ = json.NewEncoder(os.Stdout).Encode(out)
		os.Exit(0)
	case d.isBlock():
		fmt.Fprintln(os.Stderr, "BLOCKED: "+d.message())
		os.Exit(2)
	}
	os.Exit(0)
}
