package main

import (
	"testing"
)

// makeTestConfig returns a Configuration representative of a typical pypi build.
func makeTestConfig() Configuration {
	return Configuration{
		Package: PackageConfiguration{
			Name:   "my-package",
			Script: "my-script",
			Version: PackageVersionConfiguration{
				AutoUpdate: true,
				Static:     "1.0.0",
			},
		},
	}
}

// TestComputeIdentifier_StableForPypi is the regression guard for
// "additive only, don't affect existing features". The expected value
// was captured from the pre-git-support version of ComputeIdentifier
// and must not change when GIT_SOURCE is empty.
func TestComputeIdentifier_StableForPypi(t *testing.T) {
	// Ensure GIT_SOURCE is empty for this test (it's the default, but be explicit).
	origGitSource := GIT_SOURCE
	GIT_SOURCE = ""
	t.Cleanup(func() { GIT_SOURCE = origGitSource })

	cfg := makeTestConfig()
	got := cfg.ComputeIdentifier()
	want := "my-package-30bb8d607d1417531f6df2a733f8ad02439c2b3a"
	if got != want {
		t.Fatalf("ComputeIdentifier() = %q, want %q (regression: existing pypi/wheel binaries must hash identically)", got, want)
	}
}

func TestComputeIdentifier_DiffersByGitSource(t *testing.T) {
	cfg := makeTestConfig()

	origGitSource := GIT_SOURCE
	t.Cleanup(func() { GIT_SOURCE = origGitSource })

	GIT_SOURCE = ""
	pypiID := cfg.ComputeIdentifier()

	GIT_SOURCE = "git+https://github.com/org/repo"
	gitID := cfg.ComputeIdentifier()

	if pypiID == gitID {
		t.Fatalf("expected git build to produce a different identifier than pypi build, both got %q", pypiID)
	}
}

func TestComputeIdentifier_DiffersByGitRef(t *testing.T) {
	cfg := makeTestConfig()

	origGitSource := GIT_SOURCE
	t.Cleanup(func() { GIT_SOURCE = origGitSource })

	GIT_SOURCE = "git+https://github.com/org/repo@main"
	mainID := cfg.ComputeIdentifier()

	GIT_SOURCE = "git+https://github.com/org/repo@v1.0.0"
	tagID := cfg.ComputeIdentifier()

	if mainID == tagID {
		t.Fatalf("expected different git refs to produce different identifiers, both got %q", mainID)
	}
}
