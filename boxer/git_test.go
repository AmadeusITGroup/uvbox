package main

import (
	"strings"
	"testing"
)

// TestBuildGoBuildLdflags_Pypi is a regression guard: the ldflag string
// produced for a plain pypi build must match what goBuild used to produce
// before git support was added.
func TestBuildGoBuildLdflags_Pypi(t *testing.T) {
	got := buildGoBuildLdflags("", nil)
	want := "-s -w"
	if got != want {
		t.Fatalf("buildGoBuildLdflags pypi = %q, want %q", got, want)
	}
}

// TestBuildGoBuildLdflags_Wheel is a regression guard for the wheel path.
func TestBuildGoBuildLdflags_Wheel(t *testing.T) {
	got := buildGoBuildLdflags("", []string{"some-wheel.whl"})
	want := "-s -w -X main.INSTALL_WHEELS=yes"
	if got != want {
		t.Fatalf("buildGoBuildLdflags wheel = %q, want %q", got, want)
	}
}

func TestBuildGoBuildLdflags_Git(t *testing.T) {
	got := buildGoBuildLdflags("git+https://github.com/org/repo@main", nil)
	if !strings.Contains(got, "-s -w") {
		t.Errorf("expected base flags -s -w, got %q", got)
	}
	if !strings.Contains(got, "-X main.GIT_SOURCE=git+https://github.com/org/repo@main") {
		t.Errorf("expected -X main.GIT_SOURCE, got %q", got)
	}
	if strings.Contains(got, "INSTALL_WHEELS") {
		t.Errorf("git build must not set INSTALL_WHEELS, got %q", got)
	}
}

// TestBuildGoBuildLdflags_GitDoesNotOmitOnWheels covers the edge case
// where both inputs are set. CLI validation enforces mutual exclusion at
// the command layer; this test just verifies the helper itself still emits
// the git flag if both happen to be passed.
func TestBuildGoBuildLdflags_GitDoesNotOmitOnWheels(t *testing.T) {
	got := buildGoBuildLdflags("git+https://github.com/org/repo", []string{"some.whl"})
	if !strings.Contains(got, "-X main.GIT_SOURCE=git+https://github.com/org/repo") {
		t.Errorf("expected -X main.GIT_SOURCE in output, got %q", got)
	}
}
