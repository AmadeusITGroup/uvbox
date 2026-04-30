package main

import (
	"fmt"
	"net/url"
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
// ldflags. The Go toolchain splits GOFLAGS on whitespace, so any ldflag
// string containing `-X main.FOO=bar` is broken across flag boundaries
// and the toolchain rejects `-X` as an unknown top-level flag. Instead,
// the git source is written to box/git_source.txt and embedded via
// //go:embed — see writeGitSourceFile below and box/box_package_git.go.
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
// was provided. It enforces the uvbox-side format constraints so that
// obvious typos fail at build time rather than on every end-user machine:
//
//   - must begin with "git+"
//   - the URL after the "git+" prefix must parse
//   - scheme must be one of http, https, ssh, file
//   - non-file schemes must have a host
//
// Semantic ref resolution (branch, tag, commit existence) is still
// delegated to uv on first run of the generated binary.
func validateGitSource(gitSource string) error {
	if gitSource == "" {
		return fmt.Errorf("git source must not be empty")
	}
	if !strings.HasPrefix(gitSource, "git+") {
		return fmt.Errorf("git source must start with 'git+' (e.g. git+https://github.com/org/repo), got %q", gitSource)
	}

	raw := strings.TrimPrefix(gitSource, "git+")
	if raw == "" {
		return fmt.Errorf("git source is missing a URL after 'git+' prefix, got %q", gitSource)
	}

	// Strip an optional trailing "@ref" before URL parsing, but only when
	// the `@` is not part of a userinfo segment like "ssh://git@host/...".
	// Any `@` appearing after the first `/` of the path component is an
	// "@ref" suffix; userinfo `@` always precedes the host segment.
	parseTarget := raw
	if slash := strings.Index(raw, "/"); slash != -1 {
		if at := strings.LastIndex(raw, "@"); at > slash {
			parseTarget = raw[:at]
		}
	}

	u, err := url.Parse(parseTarget)
	if err != nil {
		return fmt.Errorf("git source %q is not a valid URL: %w", gitSource, err)
	}

	switch u.Scheme {
	case "http", "https", "ssh", "file":
	default:
		return fmt.Errorf("git source scheme %q is not supported; use http(s), ssh, or file (got %q)", u.Scheme, gitSource)
	}

	if u.Scheme != "file" && u.Host == "" {
		return fmt.Errorf("git source %q is missing a host", gitSource)
	}

	return nil
}
