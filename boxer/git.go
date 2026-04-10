package main

import (
	"fmt"
	"strings"
)

// buildGoBuildLdflags constructs the ldflags string passed to `go build`
// via the `GOFLAGS=-ldflags=...` environment variable. Pure function:
// no side effects, no package-level state reads, so it can be unit-tested
// without invoking go build. The arguments mirror the inputs the caller
// has at the point of invocation (gitSource CLI arg, WheelsToEmbed list).
//
// Behavior:
//   - Always includes "-s -w" to strip debug info.
//   - If wheels are embedded, adds "-X main.INSTALL_WHEELS=yes" (unchanged
//     from the pre-git-support behavior — regression test locks this in).
//   - If a git source is set, adds "-X main.GIT_SOURCE=<spec>".
//
// The git and wheel flags are independent: validateGitSource + CLI command
// separation ensure only one of the two is actually set in practice. This
// function does not enforce mutual exclusion; the CLI layer does.
func buildGoBuildLdflags(gitSource string, wheelsToEmbed []string) string {
	ldflags := "-s -w"
	if len(wheelsToEmbed) > 0 {
		ldflags += " -X main.INSTALL_WHEELS=yes"
	}
	if gitSource != "" {
		ldflags += fmt.Sprintf(" -X main.GIT_SOURCE=%s", gitSource)
	}
	return ldflags
}

// validateGitSource is called from preRun when a GitSource CLI argument
// was provided. It enforces the only format constraint we validate in
// uvbox: the spec must begin with "git+". Everything else is delegated
// to uv, which surfaces malformed specs loudly on first run of the
// generated binary.
func validateGitSource(gitSource string) error {
	if gitSource == "" {
		return fmt.Errorf("git source must not be empty")
	}
	if !strings.HasPrefix(gitSource, "git+") {
		return fmt.Errorf("git source must start with 'git+' (e.g. git+https://github.com/org/repo), got %q", gitSource)
	}
	return nil
}
