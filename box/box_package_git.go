package main

import (
	"fmt"
	"os"
	"os/exec"
)

// GIT_SOURCE is populated via ldflags at build time (-X main.GIT_SOURCE=git+...)
// when the binary is produced by `uvbox git <spec>`. When empty, the binary was
// produced by `uvbox pypi` or `uvbox wheel` and the runtime install path skips
// the git dispatch entirely.
var GIT_SOURCE = ""

// buildUvToolInstallFromArgs constructs the command-line arguments for
// `uv tool install --from <gitSource> <packageName> --upgrade`, optionally
// appending `--with-requirements <constraintsFile>`. Pure function: no
// side effects, easy to unit-test.
//
// The `--upgrade` flag is always included for git sources because there is
// no "pinned version" concept — every install/update must re-resolve the ref.
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
// `uv tool install --from <GIT_SOURCE> <PackageName> --upgrade`. Mirrors
// uvToolInstallPypi and uvToolInstallWheels in shape and error handling,
// but uses --from to delegate git-spec parsing to uv itself.
func (b *Box) uvToolInstallGit(constraintsFile string) error {
	logger.Debug("Installing package from git",
		logger.Args("name", b.PackageName, "source", GIT_SOURCE, "constraintsFile", constraintsFile))

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
	if debugEnabled() || traceEnabled() {
		cmd.Stdout = os.Stdout
	}
	logger.Trace("Running", logger.Args("command", commandArgs, "env", env))

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to run command %v: %w", commandArgs, err)
	}

	logger.Debug("Installed", logger.Args("package", b.PackageName))
	return nil
}
