// Command ghbot runs any gh command under a GitHub App (bot) identity.
//
// It mints an installation access token from a GitHub App private key stored in
// the macOS keychain, then runs `gh <args...>` with that token via GH_TOKEN, so
// the command acts as the app[bot] rather than the local gh login. Arguments are
// passed straight through to gh.
//
//	ghbot pr create --fill
//	ghbot issue comment 5 --body "thanks"
//	ghbot api repos/owner/repo/issues -f title=hi
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sudame/claude-bash-guard/internal/botgh"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: ghbot <gh command and args>")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	token, err := botgh.Token(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ghbot: "+err.Error())
		os.Exit(1)
	}

	code, err := botgh.ExecGh(token, os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "ghbot: "+err.Error())
		os.Exit(1)
	}
	os.Exit(code)
}
