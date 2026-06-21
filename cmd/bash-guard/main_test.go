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
			got := evaluate(cmd, config.Config{})

			// Then the decision matches the expected value
			assert.Equal(t, tt.want, got, "cmd=%q", cmd)
		})
	}
}

func TestDisabledRules(t *testing.T) {
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
		{"disabling one rule leaves others active", "cd /tmp", config.Config{DisabledRules: []string{ruleChaining}}, blockCd},
		{"unknown rule id is ignored", "cd /tmp", config.Config{DisabledRules: []string{"bogus"}}, blockCd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// When evaluate is called with the rule disabled
			got := evaluate(tt.cmd, tt.cfg)

			// Then the decision matches the expected value
			assert.Equal(t, tt.want, got, "cmd=%q", tt.cmd)
		})
	}
}
