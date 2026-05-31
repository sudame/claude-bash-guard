package main

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sudame/claude-bash-guard/internal/config"
)

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name string
		cmd  string
		want decision
	}{
		{"plain ls is allowed", "ls -la", allow},
		{"&& chaining is blocked", "echo a && echo b", blockChaining},
		{"; chaining is blocked", "echo a; echo b", blockChaining},
		{"chaining inside single quotes is allowed", "echo 'a && b'", allow},
		{"chaining inside double quotes is allowed", "echo \"a; b\"", allow},
		{"git -C is blocked", "git -C /tmp status", blockGitDashC},
		{"plain git status is allowed", "git status", allow},
		{"cd is blocked", "cd /tmp", blockCd},
		{"bare cd is blocked", "cd", blockCd},
		{"aicd is allowed", "aicd /tmp", allow},
		{"cd inside single quotes is allowed", "echo 'cd /tmp'", allow},
		{"gh api GET (no method flag) is allowed", "gh api user", allow},
		{"gh api -X POST asks", "gh api -X POST repos/foo/bar/issues", askGhApiWrite},
		{"gh api --method DELETE asks", "gh api --method DELETE repos/foo/bar", askGhApiWrite},
		{"gh api reply to review comment is allowed", "gh api repos/foo/bar/pulls/1/comments/12345/replies -X POST -f body=ok", allow},
		{"gh api --method POST reply to review comment is allowed", "gh api repos/foo/bar/pulls/1/comments/12345/replies --method POST -f body=ok", allow},
		{"aws without --profile is blocked", "aws s3 ls", blockAwsNoProfile},
		{"aws with --profile is allowed", "aws s3 ls --profile dev", allow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a command string
			cmd := tt.cmd

			// When evaluate is called
			got := evaluate(cmd, "", config.Config{})

			// Then the decision matches the expected value
			assert.Equal(t, tt.want, got, "cmd=%q", cmd)
		})
	}
}

func TestGhPrCreateAccountGuard(t *testing.T) {
	tests := []struct {
		name   string
		cmd    string
		cwd    string
		active string
		cfg    config.Config
		want   decision
	}{
		{"matching active account is allowed", "gh pr create --fill", "/a", "sudame-bot", config.Config{Account: "sudame-bot"}, allow},
		{"mismatching active account is blocked", "gh pr create --fill", "/a", "sudame", config.Config{Account: "sudame-bot"}, blockGhPrCreateNotBot},
		{"no active account is blocked", "gh pr create", "/a", "", config.Config{Account: "sudame-bot"}, blockGhPrCreateNotBot},
		{"configurable account name", "gh pr create", "/a", "other-bot", config.Config{Account: "other-bot"}, allow},
		{"empty config account disables the check", "gh pr create", "/a", "sudame", config.Config{}, allow},
		{"excluded path skips the check", "gh pr create", "/oss/foo", "sudame", config.Config{Account: "sudame-bot", ExcludePaths: []string{"/oss"}}, allow},
		{"non-excluded path still enforced", "gh pr create", "/work/foo", "sudame", config.Config{Account: "sudame-bot", ExcludePaths: []string{"/oss"}}, blockGhPrCreateNotBot},
		{"other gh pr command is unaffected", "gh pr list", "/a", "sudame", config.Config{Account: "sudame-bot"}, allow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given a stubbed active account
			orig := activeAccountFunc
			activeAccountFunc = func() string { return tt.active }
			defer func() { activeAccountFunc = orig }()

			// When evaluate is called
			got := evaluate(tt.cmd, tt.cwd, tt.cfg)

			// Then the decision matches the expected value
			assert.Equal(t, tt.want, got, "cmd=%q active=%q", tt.cmd, tt.active)
		})
	}
}

func TestGhPrCreateBotprGuard(t *testing.T) {
	botprCfg := config.Config{Botpr: config.Botpr{AppID: 1, InstallationID: 2}}

	tests := []struct {
		name string
		cmd  string
		cwd  string
		cfg  config.Config
		want decision
	}{
		{"botpr mode blocks gh pr create with the botpr hint", "gh pr create --fill", "/a", botprCfg, blockGhPrCreateUseBotpr},
		{"botpr mode takes precedence over account mode", "gh pr create", "/a", config.Config{Account: "sudame-bot", Botpr: config.Botpr{AppID: 1, InstallationID: 2}}, blockGhPrCreateUseBotpr},
		{"excluded path skips botpr enforcement", "gh pr create", "/oss/foo", config.Config{ExcludePaths: []string{"/oss"}, Botpr: config.Botpr{AppID: 1, InstallationID: 2}}, allow},
		{"disabling the rule skips botpr enforcement", "gh pr create", "/a", config.Config{DisabledRules: []string{ruleGhPrCreate}, Botpr: config.Botpr{AppID: 1, InstallationID: 2}}, allow},
		{"botpr invocation itself is allowed", "botpr --fill", "/a", botprCfg, allow},
		{"incomplete botpr config falls back (no account) to allow", "gh pr create", "/a", config.Config{Botpr: config.Botpr{AppID: 1}}, allow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Given an active account that would fail the legacy check
			orig := activeAccountFunc
			activeAccountFunc = func() string { return "sudame" }
			defer func() { activeAccountFunc = orig }()

			// When evaluate is called
			got := evaluate(tt.cmd, tt.cwd, tt.cfg)

			// Then the decision matches the expected value
			assert.Equal(t, tt.want, got, "cmd=%q", tt.cmd)
		})
	}
}

func TestDisabledRules(t *testing.T) {
	// Stub the active account so the gh_pr_create rule never shells out.
	orig := activeAccountFunc
	activeAccountFunc = func() string { return "sudame" }
	defer func() { activeAccountFunc = orig }()

	tests := []struct {
		name string
		cmd  string
		cfg  config.Config
		want decision
	}{
		{"chaining disabled is allowed", "echo a && echo b", config.Config{DisabledRules: []string{ruleChaining}}, allow},
		{"git_dash_c disabled is allowed", "git -C /tmp status", config.Config{DisabledRules: []string{ruleGitDashC}}, allow},
		{"cd disabled is allowed", "cd /tmp", config.Config{DisabledRules: []string{ruleCd}}, allow},
		{"gh_api_write disabled is allowed", "gh api -X POST repos/foo/bar/issues", config.Config{DisabledRules: []string{ruleGhApiWrite}}, allow},
		{"aws_no_profile disabled is allowed", "aws s3 ls", config.Config{DisabledRules: []string{ruleAwsNoProfile}}, allow},
		{"gh_pr_create disabled is allowed", "gh pr create", config.Config{Account: "sudame-bot", DisabledRules: []string{ruleGhPrCreate}}, allow},
		{"disabling one rule leaves others active", "cd /tmp", config.Config{DisabledRules: []string{ruleChaining}}, blockCd},
		{"unknown rule id is ignored", "cd /tmp", config.Config{DisabledRules: []string{"bogus"}}, blockCd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When evaluate is called with the rule disabled
			got := evaluate(tt.cmd, "", tt.cfg)

			// Then the decision matches the expected value
			assert.Equal(t, tt.want, got, "cmd=%q", tt.cmd)
		})
	}
}
