package main

import (
	"strings"
	"testing"
)

// TestBuildGoflagsEnv_WheelsMode is the regression guard for the bug
// introduced in f19680f (PR #14): an ldflags string containing "-X" was
// fed into GOFLAGS unquoted, which Go's parser split on whitespace and
// rejected with "unknown flag -X". The output MUST wrap the entry in
// single quotes so Go's GOFLAGS parser (cmd/internal/quoted.Split) keeps
// the whole ldflags expression as one token.
func TestBuildGoflagsEnv_WheelsMode(t *testing.T) {
	got, err := buildGoflagsEnv("-s -w -X main.INSTALL_WHEELS=yes")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "GOFLAGS='-ldflags=-s -w -X main.INSTALL_WHEELS=yes'"
	if got != want {
		t.Fatalf("\n  got:  %q\n  want: %q", got, want)
	}
}

func TestBuildGoflagsEnv_PypiMode(t *testing.T) {
	got, err := buildGoflagsEnv("-s -w")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "GOFLAGS='-ldflags=-s -w'"
	if got != want {
		t.Fatalf("\n  got:  %q\n  want: %q", got, want)
	}
}

func TestBuildGoflagsEnv_RejectsSingleQuote(t *testing.T) {
	_, err := buildGoflagsEnv("-s -w -X main.NOTE=it's-broken")
	if err == nil {
		t.Fatal("expected error for ldflags containing a single quote")
	}
	if !strings.Contains(err.Error(), "single quote") {
		t.Fatalf("error should mention single quote, got: %v", err)
	}
}
