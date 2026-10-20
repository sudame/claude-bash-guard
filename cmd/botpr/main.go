// Command botpr creates a pull request authored by a GitHub App (bot) identity.
//
// It is a thin specialization of ghbot: it mints an installation access token
// and runs `gh pr create <args...>` with that token, so the resulting PR is
// authored by the app[bot]. Arguments are passed straight through to
// `gh pr create`. Commits keep their local git authorship; only the PR author
// becomes the bot, which is what lets you review and approve it yourself.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sudame/claude-bash-guard/internal/botgh"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token, err := botgh.Token(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "botpr: "+err.Error())
		os.Exit(1)
	}

	code, err := botgh.ExecGh(token, append([]string{"pr", "create"}, os.Args[1:]...))
	if err != nil {
		fmt.Fprintln(os.Stderr, "botpr: "+err.Error())
		os.Exit(1)
	}
	os.Exit(code)
}
