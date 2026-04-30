package main

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// gitSourceContent is the raw content of git_source.txt, embedded at build
// time. For `uvbox git <spec>` builds, boxer writes the git spec into this
// file before invoking `go build`. For pypi/wheel builds, the file is
// created empty by box/generate.go (it is gitignored — not a committed
// file) so the //go:embed directive always has a target.
//
// We use a file-embed instead of an ldflag (-X main.GIT_SOURCE=...) because
// the Go toolchain splits GOFLAGS on whitespace, so any ldflag value
// containing spaces (or `-X main.FOO=bar` that uses an `=` plus another
// flag) breaks parsing. The boxer build path routes ldflags through GOFLAGS
// for Windows compatibility (see issue AmadeusITGroup/uvbox#7 and the
// comment above buildGoBuildLdflags in boxer/git.go).
//
//go:embed git_source.txt
var gitSourceContent string

// GIT_SOURCE is the embedded git source string (e.g., "git+https://github.com/org/repo@main")
// for binaries produced by `uvbox git <spec>`. It is empty for pypi/wheel
// builds, in which case the runtime install path skips the git dispatch
// entirely. strings.TrimSpace tolerates a trailing newline if the writer
// ever grows one.
var GIT_SOURCE = strings.TrimSpace(gitSourceContent)

// buildUvToolInstallFromArgs constructs the command-line arguments for
// `uv tool install --from <gitSource> <packageName> --upgrade`, optionally
// appending `--with-requirements <constraintsFile>`. Pulled out as a
// standalone helper so it can be tested without invoking `uv`.
//
// `--upgrade` is always included because uv has no cached version identity
// for a git install — the ref in the spec is opaque to uv's upgrade check,
// so forcing the re-install path is the only way to pick up new commits
// for a moving ref like `@main`. Whether this function is *called* at all
// is still gated by the outer version-check path in box/main.go — see the
// README section "Behavior of [package.version] for git builds".
//
// Constraints apply to the transitive dependency resolution — the primary
// package is pinned by the git ref itself.
func buildUvToolInstallFromArgs(uvPath, gitSource, packageName, constraintsFile string) []string {
	args := []string{
		uvPath,
		"--quiet",
		"tool",
		"install",
		"--from",
		gitSource,
		packageName,
		"--upgrade",
	}
	if constraintsFile != "" {
		args = append(args, "--with-requirements", constraintsFile)
	}
	return args
}

// uvToolInstallGit installs the package from the embedded GIT_SOURCE via
// `uv tool install --from <GIT_SOURCE> <PackageName> --upgrade`.
//
// packageVersion is accepted for signature parity with uvToolInstallPypi,
// but is intentionally not used to construct the install spec: the git ref
// in GIT_SOURCE is the source of truth. If the caller supplies a non-empty
// packageVersion (i.e., the user set [package.version].static on a git
// build), a user-visible warning is emitted so the mismatch is not silent.
func (b *Box) uvToolInstallGit(packageVersion, constraintsFile string) error {
	if packageVersion != "" {
		logger.Warn("Ignoring [package.version] for git source; the git ref in GIT_SOURCE is the source of truth",
			logger.Args("ignoredVersion", packageVersion, "gitSource", GIT_SOURCE))
	}

	logger.Debug("Installing package",
		logger.Args("name", b.PackageName, "source", GIT_SOURCE, "constraintsFile", constraintsFile, "method", "git"))

	uv, err := b.InstalledUvExecutablePath()
	if err != nil {
		return fmt.Errorf("could not find uv executable: %w", err)
	}

	commandArgs := buildUvToolInstallFromArgs(uv, GIT_SOURCE, b.PackageName, constraintsFile)

	env, err := b.commandsEnvironment()
	if err != nil {
		return fmt.Errorf("could not get uv environment variables: %w", err)
	}

	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	cmd.Env = env
	cmd.Stderr = os.Stderr
	// Enable Stdout if debug is enabled
	if debugEnabled() || traceEnabled() {
		cmd.Stdout = os.Stdout
	}
	logger.Trace("Running", logger.Args("command", commandArgs, "env", env))

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run command %v: %w", commandArgs, err)
	}

	logger.Debug("Installed", logger.Args("package", b.PackageName))
	return nil
}
