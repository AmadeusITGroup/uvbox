package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// buildGoBuildLdflags constructs the ldflags string passed to `go build`
// via the `GOFLAGS=-ldflags=...` environment variable. Pure function:
// no side effects, no package-level state reads, so it can be unit-tested
// without invoking go build.
//
// Behavior:
//   - Always includes "-s -w" to strip debug info.
//   - If wheels are embedded, adds "-X main.INSTALL_WHEELS=yes" (unchanged
//     from the pre-git-support behavior — regression test locks this in).
//
// Note on git source: unlike wheels, the git source is NOT injected via
// ldflags. Go's GOFLAGS parser (strings.Fields) does not support spaces
// inside flag values, so `-X main.GIT_SOURCE=<spec>` breaks parsing for
// any ldflag string containing `-X`. Instead, the git source is written
// to box/git_source.txt and embedded via //go:embed — see
// writeGitSourceFile below and box/box_package_git.go.
func buildGoBuildLdflags(wheelsToEmbed []string) string {
	ldflags := "-s -w"
	if len(wheelsToEmbed) > 0 {
		ldflags += " -X main.INSTALL_WHEELS=yes"
	}
	return ldflags
}

// writeGitSourceFile writes the provided git source string to
// <boxRepository>/git_source.txt, which is embedded into the compiled
// binary via //go:embed in box/box_package_git.go. The file is always
// written (even with an empty string) to satisfy the go:embed directive,
// which requires the target file to exist at build time. An empty file
// means "not a git build"; a non-empty file means "git build, this is
// the spec to pass to uv tool install --from".
func writeGitSourceFile(boxRepository, gitSource string) error {
	target := filepath.Join(boxRepository, "git_source.txt")
	if err := os.WriteFile(target, []byte(gitSource), 0644); err != nil {
		return fmt.Errorf("failed to write git source file to %s: %w", target, err)
	}
	return nil
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
