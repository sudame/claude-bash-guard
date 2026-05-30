package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
			got := evaluate(cmd, "", config{})

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
		cfg    config
		want   decision
	}{
		{"matching active account is allowed", "gh pr create --fill", "/a", "sudame-bot", config{Account: "sudame-bot"}, allow},
		{"mismatching active account is blocked", "gh pr create --fill", "/a", "sudame", config{Account: "sudame-bot"}, blockGhPrCreateNotBot},
		{"no active account is blocked", "gh pr create", "/a", "", config{Account: "sudame-bot"}, blockGhPrCreateNotBot},
		{"configurable account name", "gh pr create", "/a", "other-bot", config{Account: "other-bot"}, allow},
		{"empty config account disables the check", "gh pr create", "/a", "sudame", config{}, allow},
		{"excluded path skips the check", "gh pr create", "/oss/foo", "sudame", config{Account: "sudame-bot", ExcludePaths: []string{"/oss"}}, allow},
		{"non-excluded path still enforced", "gh pr create", "/work/foo", "sudame", config{Account: "sudame-bot", ExcludePaths: []string{"/oss"}}, blockGhPrCreateNotBot},
		{"other gh pr command is unaffected", "gh pr list", "/a", "sudame", config{Account: "sudame-bot"}, allow},
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

func TestDisabledRules(t *testing.T) {
	// Stub the active account so the gh_pr_create rule never shells out.
	orig := activeAccountFunc
	activeAccountFunc = func() string { return "sudame" }
	defer func() { activeAccountFunc = orig }()

	tests := []struct {
		name string
		cmd  string
		cfg  config
		want decision
	}{
		{"chaining disabled is allowed", "echo a && echo b", config{DisabledRules: []string{ruleChaining}}, allow},
		{"git_dash_c disabled is allowed", "git -C /tmp status", config{DisabledRules: []string{ruleGitDashC}}, allow},
		{"cd disabled is allowed", "cd /tmp", config{DisabledRules: []string{ruleCd}}, allow},
		{"gh_api_write disabled is allowed", "gh api -X POST repos/foo/bar/issues", config{DisabledRules: []string{ruleGhApiWrite}}, allow},
		{"aws_no_profile disabled is allowed", "aws s3 ls", config{DisabledRules: []string{ruleAwsNoProfile}}, allow},
		{"gh_pr_create disabled is allowed", "gh pr create", config{Account: "sudame-bot", DisabledRules: []string{ruleGhPrCreate}}, allow},
		{"disabling one rule leaves others active", "cd /tmp", config{DisabledRules: []string{ruleChaining}}, blockCd},
		{"unknown rule id is ignored", "cd /tmp", config{DisabledRules: []string{"bogus"}}, blockCd},
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
