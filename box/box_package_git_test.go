package main

import (
	"reflect"
	"testing"
)

func TestBuildUvToolInstallFromArgs_Minimal(t *testing.T) {
	got := buildUvToolInstallFromArgs("/path/to/uv", "git+https://github.com/org/repo", "mypkg", "")
	want := []string{
		"/path/to/uv",
		"--quiet",
		"tool",
		"install",
		"--from",
		"git+https://github.com/org/repo",
		"mypkg",
		"--upgrade",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildUvToolInstallFromArgs minimal = %v, want %v", got, want)
	}
}

func TestBuildUvToolInstallFromArgs_WithRef(t *testing.T) {
	got := buildUvToolInstallFromArgs("uv", "git+https://github.com/org/repo@v1.0.0", "mypkg", "")
	want := []string{
		"uv",
		"--quiet",
		"tool",
		"install",
		"--from",
		"git+https://github.com/org/repo@v1.0.0",
		"mypkg",
		"--upgrade",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildUvToolInstallFromArgs with ref = %v, want %v", got, want)
	}
}

func TestBuildUvToolInstallFromArgs_WithConstraints(t *testing.T) {
	got := buildUvToolInstallFromArgs("uv", "git+https://github.com/org/repo", "mypkg", "/tmp/constraints.txt")
	want := []string{
		"uv",
		"--quiet",
		"tool",
		"install",
		"--from",
		"git+https://github.com/org/repo",
		"mypkg",
		"--upgrade",
		"--with-requirements",
		"/tmp/constraints.txt",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildUvToolInstallFromArgs with constraints = %v, want %v", got, want)
	}
}
