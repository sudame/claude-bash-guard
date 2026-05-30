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
			got := evaluate(cmd)

			// Then the decision matches the expected value
			assert.Equal(t, tt.want, got, "cmd=%q", cmd)
		})
	}
}
