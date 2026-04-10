package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestBuildGoBuildLdflags_Pypi is a regression guard: the ldflag string
// produced for a plain pypi build must match what goBuild used to produce
// before git support was added. Git support does NOT inject ldflags
// (see boxer/git.go for why) so adding it must not change this output.
func TestBuildGoBuildLdflags_Pypi(t *testing.T) {
	got := buildGoBuildLdflags(nil)
	want := "-s -w"
	if got != want {
		t.Fatalf("buildGoBuildLdflags pypi = %q, want %q", got, want)
	}
}

// TestBuildGoBuildLdflags_Wheel is a regression guard for the wheel path.
func TestBuildGoBuildLdflags_Wheel(t *testing.T) {
	got := buildGoBuildLdflags([]string{"some-wheel.whl"})
	want := "-s -w -X main.INSTALL_WHEELS=yes"
	if got != want {
		t.Fatalf("buildGoBuildLdflags wheel = %q, want %q", got, want)
	}
}

func TestWriteGitSourceFile_Empty(t *testing.T) {
	dir := t.TempDir()
	if err := writeGitSourceFile(dir, ""); err != nil {
		t.Fatalf("writeGitSourceFile empty failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "git_source.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(got) != "" {
		t.Fatalf("expected empty file, got %q", string(got))
	}
}

func TestWriteGitSourceFile_WithSpec(t *testing.T) {
	dir := t.TempDir()
	spec := "git+https://github.com/org/repo@main"

	if err := writeGitSourceFile(dir, spec); err != nil {
		t.Fatalf("writeGitSourceFile failed: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, "git_source.txt"))
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(got) != spec {
		t.Fatalf("expected %q, got %q", spec, string(got))
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
