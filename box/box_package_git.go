package main

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
