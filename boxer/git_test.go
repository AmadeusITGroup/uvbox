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

func TestValidateGitSource_Empty(t *testing.T) {
	if err := validateGitSource(""); err == nil {
		t.Fatal("expected error for empty git source, got nil")
	}
}

func TestValidateGitSource_MissingGitPrefix(t *testing.T) {
	cases := []string{
		"https://github.com/org/repo",
		"http://github.com/org/repo",
		"github.com/org/repo",
		"ssh://git@github.com/org/repo",
		"file:///tmp/repo",
	}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			if err := validateGitSource(s); err == nil {
				t.Fatalf("expected error for %q, got nil", s)
			}
		})
	}
}

func TestValidateGitSource_AcceptsValidSpecs(t *testing.T) {
	cases := []string{
		"git+https://github.com/org/repo",
		"git+https://github.com/org/repo@main",
		"git+https://github.com/org/repo@v1.0.0",
		"git+https://github.com/org/repo@abc123def456",
		"git+ssh://git@github.com/org/repo",
		"git+ssh://git@github.com/org/repo@main",
	}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			if err := validateGitSource(s); err != nil {
				t.Fatalf("expected %q to be valid, got error: %v", s, err)
			}
		})
	}
}
