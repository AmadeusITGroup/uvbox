package main

import (
	"os"
	"path/filepath"
	"testing"
)

func hideStdoutAndStderr() {
	hideStdout()
	hideStderr()
}

func hideStdout() {
	os.Stdout, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
}

func hideStderr() {
	os.Stderr, _ = os.OpenFile(os.DevNull, os.O_WRONLY, 0)
}

func testDirectory(t *testing.T) string {
	// Create temporary directory
	folder, err := os.MkdirTemp("", "uvbox-test-")
	if err != nil {
		t.Fatalf("could not create temporary directory: %v", err)
	}
	return folder
}

func installedTextBoxWithDummyPackage(t *testing.T, homePath, identifier string) *Box {
	return installedTestBox(t, homePath, identifier, "myPackage")
}

func installedTextBoxDummy(t *testing.T, homePath string) *Box {
	return installedTestBox(t, homePath, "myIdentifier", "myPackage")
}

func installedTestBox(t *testing.T, homePath, identifier string, packageName string) *Box {
	box := GetBox(
		identifier,
		packageName,
		[]string{},
		homePath,
	)

	err := box.Install()
	if err != nil {
		t.Fatal(err)
	}

	return box
}

func TestCreateUvboxFolder(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	installedTextBoxDummy(t, homePath)

	expectedPath := filepath.Join(homePath, "uvbox")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("expected folder %s to exist", expectedPath)
	} else if err != nil {
		t.Fatalf("could not check if folder %s exists: %v", expectedPath, err)
	}
}

func TestCanTellIfNeedsInstall(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	box := GetBox("myIdentifier", "myPackage", []string{}, homePath)

	if yes, err := box.NeedsInstall(); err != nil {
		t.Fatalf("could not check if needs install: %v", err)
	} else if !yes {
		t.Fatalf("expected to need install")
	}
}

func TestCanTellIfDoesNotNeedInstall(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	box := installedTextBoxDummy(t, homePath)

	if yes, err := box.NeedsInstall(); err != nil {
		t.Fatalf("could not check if needs install: %v", err)
	} else if yes {
		t.Fatalf("expected to not need install")
	}
}

func TestCanInstallWithSpinner(t *testing.T) {
	hideStdoutAndStderr()

	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	box := GetBox("myIdentifier", "myPackage", []string{}, homePath)

	err := box.InstallWithSpinner()
	if err != nil {
		t.Fatalf("could not install: %v", err)
	}

	if yes, err := box.NeedsInstall(); err != nil {
		t.Fatalf("could not check if needs install: %v", err)
	} else if yes {
		t.Fatalf("expected to not need install")
	}
}

func TestCanUninstall(t *testing.T) {
	hideStdoutAndStderr()

	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	box := installedTextBoxWithPackage(t, homePath, "cowsay", "6.1", "")

	err := box.Uninstall()
	if err != nil {
		t.Fatalf("could not uninstall box: %v", err)
	}

	if yes, err := box.uvboxConfigurationInstalled(); err != nil {
		t.Fatalf("could not check if needs configuration needs installation: %v", err)
	} else if yes {
		t.Fatalf("expected configuration file to need installation after uninstallation")
	}

	if yes, err := box.certificatesBundleInstalled(); err != nil {
		t.Fatalf("could not check if needs certificates bundle file needs installation: %v", err)
	} else if yes {
		t.Fatalf("expected certificates bundle file to need installation after uninstallation")
	}

	if yes, err := box.IsPackageInstalled(); err != nil {
		t.Fatalf("could not check if package needs installation: %v", err)
	} else if yes {
		t.Fatalf("expected package to need installation after uninstallation")
	}
}

func TestCanUninstallWithoutPriorInstallWithoutCrash(t *testing.T) {
	hideStdoutAndStderr()

	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	box := GetBox("myIdentifier", "myPackage", []string{}, homePath)

	err := box.Uninstall()
	if err != nil {
		t.Fatalf("could not uninstall box: %v", err)
	}
}

// Test filterOutPythonEnvironmentVariables filters UV_* variables
func TestFilterOutPythonEnvironmentVariablesFiltersUvVariables(t *testing.T) {
	input := []string{
		"UV_CACHE_DIR=/some/path",
		"UV_TOOL_DIR=/another/path",
		"UV_NO_CONFIG=1",
		"UV_PYTHON=3.12",
		"PATH=/usr/bin",
		"HOME=/home/user",
	}

	filtered, removed := filterOutPythonEnvironmentVariables(input)

	// Check filtered contains non-UV variables
	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered variables, got %d: %v", len(filtered), filtered)
	}
	if filtered[0] != "PATH=/usr/bin" || filtered[1] != "HOME=/home/user" {
		t.Fatalf("expected PATH and HOME to be preserved, got: %v", filtered)
	}

	// Check removed contains UV variables
	if len(removed) != 4 {
		t.Fatalf("expected 4 removed variables, got %d: %v", len(removed), removed)
	}
	expectedRemoved := map[string]bool{
		"UV_CACHE_DIR": true,
		"UV_TOOL_DIR":  true,
		"UV_NO_CONFIG": true,
		"UV_PYTHON":    true,
	}
	for _, key := range removed {
		if !expectedRemoved[key] {
			t.Fatalf("unexpected removed key: %s", key)
		}
	}
}

// Test filterOutPythonEnvironmentVariables filters VIRTUAL_ENV
func TestFilterOutPythonEnvironmentVariablesFiltersVirtualEnv(t *testing.T) {
	input := []string{
		"VIRTUAL_ENV=/home/user/.venv",
		"PATH=/usr/bin",
		"HOME=/home/user",
	}

	filtered, removed := filterOutPythonEnvironmentVariables(input)

	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered variables, got %d: %v", len(filtered), filtered)
	}
	if len(removed) != 1 || removed[0] != "VIRTUAL_ENV" {
		t.Fatalf("expected VIRTUAL_ENV to be removed, got: %v", removed)
	}
}

// Test filterOutPythonEnvironmentVariables filters PYTHONPATH
func TestFilterOutPythonEnvironmentVariablesFiltersPythonpath(t *testing.T) {
	input := []string{
		"PYTHONPATH=/home/user/lib:/home/user/packages",
		"PATH=/usr/bin",
		"HOME=/home/user",
	}

	filtered, removed := filterOutPythonEnvironmentVariables(input)

	if len(filtered) != 2 {
		t.Fatalf("expected 2 filtered variables, got %d: %v", len(filtered), filtered)
	}
	if len(removed) != 1 || removed[0] != "PYTHONPATH" {
		t.Fatalf("expected PYTHONPATH to be removed, got: %v", removed)
	}
}

// Test filterOutPythonEnvironmentVariables filters all Python-related variables together
func TestFilterOutPythonEnvironmentVariablesFiltersAllPythonVariables(t *testing.T) {
	input := []string{
		"UV_CACHE_DIR=/cache",
		"UV_TOOL_DIR=/tools",
		"UV_TOOL_BIN_DIR=/tools-bin",
		"VIRTUAL_ENV=/home/user/.venv",
		"PYTHONPATH=/home/user/lib",
		"PATH=/usr/bin",
		"HOME=/home/user",
		"LANG=en_US.UTF-8",
	}

	filtered, removed := filterOutPythonEnvironmentVariables(input)

	// Check filtered preserves non-Python variables
	if len(filtered) != 3 {
		t.Fatalf("expected 3 filtered variables, got %d: %v", len(filtered), filtered)
	}
	expectedFiltered := []string{"PATH=/usr/bin", "HOME=/home/user", "LANG=en_US.UTF-8"}
	for i, expected := range expectedFiltered {
		if filtered[i] != expected {
			t.Fatalf("expected filtered[%d] = %s, got %s", i, expected, filtered[i])
		}
	}

	// Check removed contains all Python-related variables
	if len(removed) != 5 {
		t.Fatalf("expected 5 removed variables, got %d: %v", len(removed), removed)
	}
	expectedRemoved := map[string]bool{
		"UV_CACHE_DIR":    true,
		"UV_TOOL_DIR":     true,
		"UV_TOOL_BIN_DIR": true,
		"VIRTUAL_ENV":     true,
		"PYTHONPATH":      true,
	}
	for _, key := range removed {
		if !expectedRemoved[key] {
			t.Fatalf("unexpected removed key: %s", key)
		}
	}
}

// Test filterOutPythonEnvironmentVariables with empty input
func TestFilterOutPythonEnvironmentVariablesEmptyInput(t *testing.T) {
	input := []string{}

	filtered, removed := filterOutPythonEnvironmentVariables(input)

	if len(filtered) != 0 {
		t.Fatalf("expected 0 filtered variables, got %d", len(filtered))
	}
	if len(removed) != 0 {
		t.Fatalf("expected 0 removed variables, got %d", len(removed))
	}
}

// Test filterOutPythonEnvironmentVariables when nothing needs filtering
func TestFilterOutPythonEnvironmentVariablesNoMatches(t *testing.T) {
	input := []string{
		"PATH=/usr/bin",
		"HOME=/home/user",
		"LANG=en_US.UTF-8",
		"TERM=xterm-256color",
	}

	filtered, removed := filterOutPythonEnvironmentVariables(input)

	if len(filtered) != 4 {
		t.Fatalf("expected 4 filtered variables, got %d: %v", len(filtered), filtered)
	}
	if len(removed) != 0 {
		t.Fatalf("expected 0 removed variables, got %d: %v", len(removed), removed)
	}
	for i, expected := range input {
		if filtered[i] != expected {
			t.Fatalf("expected filtered[%d] = %s, got %s", i, expected, filtered[i])
		}
	}
}

// Test filterOutPythonEnvironmentVariables does not filter variables that start with UV but not UV_
func TestFilterOutPythonEnvironmentVariablesDoesNotFilterUvWithoutUnderscore(t *testing.T) {
	input := []string{
		"UVBOX_DEBUG=1",
		"UVISION=enabled",
		"UV_CACHE_DIR=/cache",
		"PATH=/usr/bin",
	}

	filtered, removed := filterOutPythonEnvironmentVariables(input)

	// UVBOX_DEBUG and UVISION should be preserved (they don't start with UV_)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 filtered variables, got %d: %v", len(filtered), filtered)
	}
	if filtered[0] != "UVBOX_DEBUG=1" || filtered[1] != "UVISION=enabled" || filtered[2] != "PATH=/usr/bin" {
		t.Fatalf("expected UVBOX_DEBUG, UVISION, and PATH to be preserved, got: %v", filtered)
	}

	// Only UV_CACHE_DIR should be removed
	if len(removed) != 1 || removed[0] != "UV_CACHE_DIR" {
		t.Fatalf("expected only UV_CACHE_DIR to be removed, got: %v", removed)
	}
}

// Test filterOutPythonEnvironmentVariables handles variables without values
func TestFilterOutPythonEnvironmentVariablesHandlesEmptyValues(t *testing.T) {
	input := []string{
		"UV_CACHE_DIR=",
		"VIRTUAL_ENV=",
		"PATH=",
	}

	filtered, removed := filterOutPythonEnvironmentVariables(input)

	if len(filtered) != 1 || filtered[0] != "PATH=" {
		t.Fatalf("expected PATH= to be preserved, got: %v", filtered)
	}
	if len(removed) != 2 {
		t.Fatalf("expected 2 removed variables, got %d: %v", len(removed), removed)
	}
}

// Test commandsEnvironment filters inherited UV_* variables
func TestCommandsEnvironmentFiltersInheritedUvVariables(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	// Set some UV_* environment variables that simulate a parent uvbox binary
	// t.Setenv automatically restores the original value after the test
	t.Setenv("UV_CACHE_DIR", "/parent/cache")
	t.Setenv("UV_TOOL_DIR", "/parent/tools")
	t.Setenv("UV_TOOL_BIN_DIR", "/parent/tools-bin")
	t.Setenv("UV_PYTHON_INSTALL_DIR", "/parent/python")
	t.Setenv("UV_NO_CONFIG", "1")

	box := GetBox("childIdentifier", "childPackage", []string{}, homePath)
	env, err := box.commandsEnvironment()
	if err != nil {
		t.Fatalf("could not get commands environment: %v", err)
	}

	// Build a map for easy lookup
	envMap := make(map[string]string)
	for _, e := range env {
		if idx := indexOf(e, "="); idx != -1 {
			key := e[:idx]
			value := e[idx+1:]
			// Only keep the first occurrence (which should be the box's own variables)
			if _, exists := envMap[key]; !exists {
				envMap[key] = value
			}
		}
	}

	// Check that the box's OWN UV_* variables are present (not the parent's)
	expectedCacheDir := filepath.Join(homePath, "uvbox", "childIdentifier", "cache")
	if envMap["UV_CACHE_DIR"] != expectedCacheDir {
		t.Fatalf("expected UV_CACHE_DIR=%s, got %s", expectedCacheDir, envMap["UV_CACHE_DIR"])
	}

	expectedToolDir := filepath.Join(homePath, "uvbox", "childIdentifier", "tools")
	if envMap["UV_TOOL_DIR"] != expectedToolDir {
		t.Fatalf("expected UV_TOOL_DIR=%s, got %s", expectedToolDir, envMap["UV_TOOL_DIR"])
	}

	// Verify the parent's values are NOT present anywhere in the env list
	for _, e := range env {
		if e == "UV_CACHE_DIR=/parent/cache" {
			t.Fatal("parent's UV_CACHE_DIR should have been filtered out")
		}
		if e == "UV_TOOL_DIR=/parent/tools" {
			t.Fatal("parent's UV_TOOL_DIR should have been filtered out")
		}
	}
}

// Test commandsEnvironment filters VIRTUAL_ENV and PYTHONPATH
func TestCommandsEnvironmentFiltersVirtualEnvAndPythonpath(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	// Set VIRTUAL_ENV and PYTHONPATH that simulate a parent environment
	// t.Setenv automatically restores the original value after the test
	t.Setenv("VIRTUAL_ENV", "/parent/.venv")
	t.Setenv("PYTHONPATH", "/parent/lib:/parent/packages")

	box := GetBox("childIdentifier", "childPackage", []string{}, homePath)
	env, err := box.commandsEnvironment()
	if err != nil {
		t.Fatalf("could not get commands environment: %v", err)
	}

	// Verify VIRTUAL_ENV and PYTHONPATH are NOT present in the environment
	for _, e := range env {
		if e == "VIRTUAL_ENV=/parent/.venv" {
			t.Fatal("VIRTUAL_ENV should have been filtered out")
		}
		if e == "PYTHONPATH=/parent/lib:/parent/packages" {
			t.Fatal("PYTHONPATH should have been filtered out")
		}
	}
}

// Test commandsEnvironment preserves non-Python environment variables
func TestCommandsEnvironmentPreservesNonPythonVariables(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	// Set some environment variables that should be preserved
	// t.Setenv automatically restores the original value after the test
	testValue := "test-value-12345"
	t.Setenv("MY_CUSTOM_VAR", testValue)

	box := GetBox("testIdentifier", "testPackage", []string{}, homePath)
	env, err := box.commandsEnvironment()
	if err != nil {
		t.Fatalf("could not get commands environment: %v", err)
	}

	// Check that MY_CUSTOM_VAR is present
	found := false
	for _, e := range env {
		if e == "MY_CUSTOM_VAR="+testValue {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("MY_CUSTOM_VAR should have been preserved in the environment")
	}
}

// Test commandsEnvironment includes box's UV extra environment variables
func TestCommandsEnvironmentIncludesUvExtraEnviron(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	uvExtraEnviron := []string{
		"UV_PYTHON=3.12",
		"UV_INDEX=https://custom.pypi.org/simple",
	}

	box := GetBox("testIdentifier", "testPackage", uvExtraEnviron, homePath)
	env, err := box.commandsEnvironment()
	if err != nil {
		t.Fatalf("could not get commands environment: %v", err)
	}

	// Check that UV extra environ variables are present
	foundPython := false
	foundIndex := false
	for _, e := range env {
		if e == "UV_PYTHON=3.12" {
			foundPython = true
		}
		if e == "UV_INDEX=https://custom.pypi.org/simple" {
			foundIndex = true
		}
	}
	if !foundPython {
		t.Fatal("UV_PYTHON from uvExtraEnviron should be present")
	}
	if !foundIndex {
		t.Fatal("UV_INDEX from uvExtraEnviron should be present")
	}
}

// Helper function to find index of substring
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
