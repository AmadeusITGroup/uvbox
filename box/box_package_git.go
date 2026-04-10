package main

// GIT_SOURCE is populated via ldflags at build time (-X main.GIT_SOURCE=git+...)
// when the binary is produced by `uvbox git <spec>`. When empty, the binary was
// produced by `uvbox pypi` or `uvbox wheel` and the runtime install path skips
// the git dispatch entirely.
var GIT_SOURCE = ""
