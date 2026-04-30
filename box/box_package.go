package main

import (
	"embed"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/theckman/yacspin"
)

// INSTALL_WHEELS is a build flag that enables installing wheels instead of using pypi.
var INSTALL_WHEELS = "no"

//go:embed wheels
var wheelFiles embed.FS // Contains a wheels folder with embedded wheel files

func (b *Box) InstallPackageWithSpinner(packageVersion, packageConstraintsUrl string) error {
	cfg := yacspin.Config{
		Frequency:       50 * time.Millisecond,
		CharSet:         yacspin.CharSets[14],
		Suffix:          " 📦 Setting up package...",
		SuffixAutoColon: true,
	}
	spinner, _ := yacspin.New(cfg)

	spinnerErr := spinner.Start()
	if spinnerErr != nil {
		fmt.Printf("Failed to start spinner: %v\n", spinnerErr)
	}
	err := b.InstallPackage(packageVersion, packageConstraintsUrl)
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("Failed to install %s", b.PackageName))
		spinnerErr := spinner.StopFail()
		if spinnerErr != nil {
			fmt.Printf("Failed to stop spinner: %v\n", spinnerErr)
		}
		return fmt.Errorf("failed to install package %s: %v", b.PackageName, err)
	} else {
		if spinnerErr == nil {
			spinnerErr = spinner.Stop()
			if spinnerErr != nil {
				fmt.Printf("Failed to stop spinner: %v\n", err)
			}
		}
	}

	return nil
}

func (b *Box) UpdatePackageWithSpinner(packageCurrentVersion, packageNewVersion, packageConstraintsUrl string) error {
	cfg := yacspin.Config{
		Frequency:       50 * time.Millisecond,
		CharSet:         yacspin.CharSets[14],
		Suffix:          fmt.Sprintf(" 🚀 Updating from %s to %s...", packageCurrentVersion, packageNewVersion),
		SuffixAutoColon: true,
	}
	spinner, _ := yacspin.New(cfg)

	spinnerErr := spinner.Start()
	if spinnerErr != nil {
		fmt.Printf("Failed to start spinner: %v\n", spinnerErr)
	}

	err := b.InstallPackage(packageNewVersion, packageConstraintsUrl)
	if err != nil {
		spinner.StopFailMessage(fmt.Sprintf("Failed to install %s", b.PackageName))
		spinnerErr = spinner.StopFail()
		if spinnerErr != nil {
			fmt.Printf("Failed to stop spinner: %v\n", spinnerErr)
		}
		return fmt.Errorf("failed to update package %s: %v", b.PackageName, err)
	} else {
		if spinnerErr == nil {
			spinnerErr = spinner.Stop()
			if spinnerErr != nil {
				fmt.Printf("Failed to stop spinner: %v\n", err)
			}
		}
	}

	return nil
}

func (b *Box) InstallPackage(packageVersion, packageConstraintsUrl string) error {
	constraintsFile := ""
	if packageConstraintsUrl != "" {
		file, err := downloadTemporaryFile(packageConstraintsUrl)
		if err != nil {
			logger.Debug("Failed to download constraints file", logger.Args("error", err))
		}
		constraintsFile = file
	}

	if err := b.uvToolInstall(packageVersion, constraintsFile); err != nil {
		return fmt.Errorf("failed to install package %s: %v", b.PackageName, err)
	}

	if constraintsFile != "" {
		_ = os.RemoveAll(constraintsFile)
	}
	return nil
}

func (b *Box) UninstallPackage() error {
	return b.uvToolUninstall()
}

func (b *Box) IsPackageInstalled() (bool, error) {
	uvInstalled, err := b.uvInstalled()
	if err != nil {
		return false, fmt.Errorf("failed to check if package %s is installed: %v", b.PackageName, err)
	} else if !uvInstalled {
		return false, nil
	}

	packagesVersion, err := b.uvToolList()
	if err != nil {
		return false, fmt.Errorf("failed to check if package %s is installed: %v", b.PackageName, err)
	}

	_, ok := packagesVersion[b.PackageName]
	return ok, nil
}

func (b *Box) InstalledPackageVersion() (string, error) {
	installedPackages, err := b.uvToolList()
	if err != nil {
		return "", fmt.Errorf("failed to get installed package version: %v", err)
	}

	installedPackage, ok := installedPackages[b.PackageName]
	if !ok {
		return "", fmt.Errorf("package %s is not installed", b.PackageName)
	}

	return installedPackage.Version, nil
}

func (b *Box) InstalledPackagePath() (string, error) {
	installedPackages, err := b.uvToolList()
	if err != nil {
		return "", fmt.Errorf("failed to get installed package path: %v", err)
	}

	installedPackage, ok := installedPackages[b.PackageName]
	if !ok {
		return "", fmt.Errorf("package %s is not installed", b.PackageName)
	}

	return installedPackage.Path, nil
}

// installMethod identifies which install backend uvToolInstall should use.
type installMethod int

const (
	installMethodPypi installMethod = iota
	installMethodWheels
	installMethodGit
)

// selectInstallMethod is the pure-function core of the uvToolInstall dispatch.
// It returns an error when the build-time configuration is inconsistent —
// specifically when both GIT_SOURCE and INSTALL_WHEELS are set, which would
// otherwise silently favor one source and discard the other.
func selectInstallMethod(gitSource, installWheels string) (installMethod, error) {
	gitSet := gitSource != ""
	wheelsSet := installWheels == "yes"
	if gitSet && wheelsSet {
		return 0, fmt.Errorf("invalid binary: both GIT_SOURCE and INSTALL_WHEELS are set; this indicates a build-time bug, please rebuild")
	}
	switch {
	case gitSet:
		return installMethodGit, nil
	case wheelsSet:
		return installMethodWheels, nil
	default:
		return installMethodPypi, nil
	}
}

func (b *Box) uvToolInstall(packageVersion, constraintsFile string) error {
	method, err := selectInstallMethod(GIT_SOURCE, INSTALL_WHEELS)
	if err != nil {
		return err
	}
	switch method {
	case installMethodGit:
		return b.uvToolInstallGit(packageVersion, constraintsFile)
	case installMethodWheels:
		return b.uvToolInstallWheels(constraintsFile)
	default:
		return b.uvToolInstallPypi(packageVersion, constraintsFile)
	}
}

func (b *Box) uvToolInstallWheels(constraintsFile string) error {
	// List all embedded wheel files
	entries, err := wheelFiles.ReadDir("wheels")
	if err != nil {
		return fmt.Errorf("error while reading embedded wheel: %w", err)
	}

	// Create temporary folder
	tmpDir, err := os.MkdirTemp("", "uvbox-wheels-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// Iterate on every wheel
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".whl") {
			debugArgsMap := map[string]any{
				"name":            entry.Name(),
				"version":         "",
				"constraintsFile": constraintsFile,
			}
			logger.Debug("Installing package", logger.ArgsFromMap(debugArgsMap))

			// Extract the wheel file
			destFilePath, err := extractWheelEntryInDirectory(tmpDir, entry.Name())
			if err != nil {
				return fmt.Errorf("failed to extract embedded wheel %s: %w", entry.Name(), err)
			}
			defer os.Remove(destFilePath)
			logger.Debug("Extracted wheel", logger.Args("file", destFilePath))

			// Install the wheel using uv
			if err := b.uvToolInstallLine("", constraintsFile, destFilePath); err != nil {
				return fmt.Errorf("failed to install wheel %s: %w", entry.Name(), err)
			}
		}
	}
	return nil
}

func (b *Box) uvToolInstallPypi(packageVersion, constraintsFile string) error {
	debugArgsMap := map[string]any{
		"name":            b.PackageName,
		"version":         packageVersion,
		"constraintsFile": constraintsFile,
		"method":          "pypi",
	}
	logger.Debug("Installing package", logger.ArgsFromMap(debugArgsMap))

	packageInstallationLine := ""
	if packageVersion != "" {
		packageInstallationLine = fmt.Sprintf("%s==%s", b.PackageName, packageVersion)
	} else {
		packageInstallationLine = b.PackageName
	}

	return b.uvToolInstallLine(packageVersion, constraintsFile, packageInstallationLine)
}

func (b *Box) uvToolInstallLine(packageVersion, constraintsFile, packageInstallationLine string) error {
	uv, err := b.InstalledUvExecutablePath()
	if err != nil {
		return fmt.Errorf("could not find uv executable: %w", err)
	}

	commandArgs := []string{
		uv,
		"--quiet",
		"tool",
		"install",
		packageInstallationLine,
	}

	// If no specific version is requested, we allow upgrading the package
	// This allows to update existing installations to the latest version with this function
	if packageVersion == "" {
		commandArgs = append(commandArgs, "--upgrade")
	}

	if constraintsFile != "" {
		commandArgs = append(commandArgs, "--with-requirements", constraintsFile)
	}

	env, err := b.commandsEnvironment()
	if err != nil {
		return fmt.Errorf("could not get uv environment variables: %w", err)
	}

	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	cmd.Env = env
	cmd.Stderr = os.Stderr
	// Enable Stdout if debug is enabled
	if debugEnabled() || traceEnabled() {
		cmd.Stdout = os.Stdout
	}
	logger.Trace("Running", logger.Args("command", commandArgs, "env", env))

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run command %v: %w", commandArgs, err)
	}

	logger.Debug("Installed", logger.Args("package", b.PackageName))
	return nil
}

func (b *Box) uvToolUninstall() error {
	logger.Debug("Uninstalling", logger.Args("package", b.PackageName))

	uv, err := b.InstalledUvExecutablePath()
	if err != nil {
		return fmt.Errorf("could not find uv executable: %w", err)
	}

	commandArgs := []string{
		uv,
		"--quiet",
		"tool",
		"uninstall",
		b.PackageName,
	}

	env, err := b.commandsEnvironment()
	if err != nil {
		return fmt.Errorf("could not get uv environment variables: %w", err)
	}

	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	cmd.Env = env
	cmd.Stderr = os.Stderr
	if debugEnabled() {
		cmd.Stdout = os.Stdout
	}
	logger.Trace("Running", logger.Args("command", commandArgs))

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run command %v: %w", commandArgs, err)
	}

	logger.Debug("Uninstalled", logger.Args("package", b.PackageName))
	return nil
}

func (b *Box) uvCacheClean() error {
	logger.Debug("Cleaning cache")

	uv, err := b.InstalledUvExecutablePath()
	if err != nil {
		return fmt.Errorf("could not find uv executable: %w", err)
	}

	commandArgs := []string{
		uv,
		"--quiet",
		"cache",
		"clean",
	}

	env, err := b.commandsEnvironment()
	if err != nil {
		return fmt.Errorf("could not get uv environment variables: %w", err)
	}

	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	cmd.Env = env
	cmd.Stderr = os.Stderr
	logger.Trace("Running command", logger.Args("command", commandArgs))

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to run command %v: %w", commandArgs, err)
	}

	logger.Debug("Cleaned cache")
	return nil
}

func (b *Box) uvToolList() (map[string]InstalledPackage, error) {
	uv, err := b.InstalledUvExecutablePath()
	if err != nil {
		return map[string]InstalledPackage{}, fmt.Errorf("could not find uv executable: %w", err)
	}

	commandArgs := []string{
		uv,
		"tool",
		"list",
		"--show-paths",
	}

	env, err := b.commandsEnvironment()
	if err != nil {
		return map[string]InstalledPackage{}, fmt.Errorf("could not get uv environment variables: %w", err)
	}

	cmd := exec.Command(commandArgs[0], commandArgs[1:]...)
	cmd.Env = env
	logger.Trace("Running", logger.Args("command", commandArgs))

	out, err := cmd.Output()
	if err != nil {
		return map[string]InstalledPackage{}, fmt.Errorf("failed to run command %v: %w", commandArgs, err)
	}

	logger.Trace("Command executed", logger.Args("output", string(out)))

	return parseInstalledPackages(out), nil
}

type InstalledPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Path    string `json:"path"`
}

func parseInstalledPackages(out []byte) map[string]InstalledPackage {
	installedPackages := map[string]InstalledPackage{}

	text := string(out)
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 {
		return installedPackages
	}
	for _, line := range lines {
		// Skip if starts by -
		if strings.HasPrefix("-", line) {
			continue
		}

		// First part is package name, second part is version starting by v
		words := strings.Split(line, " ")
		if len(words) < 2 {
			continue
		}
		name := strings.TrimSpace(words[0])
		version := strings.TrimPrefix(words[1], "v")
		path := strings.TrimSpace(words[2])
		path = strings.TrimPrefix(path, "(")
		path = strings.TrimSuffix(path, ")")

		installedPackages[words[0]] = InstalledPackage{
			Name:    name,
			Version: version,
			Path:    path,
		}
	}
	return installedPackages
}

func downloadTemporaryFile(url string) (string, error) {
	logger.Debug("Downloading", logger.Args("url", url))
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch file from %s: %w", url, err)
	}
	defer resp.Body.Close()

	logger.Debug("Downloaded", logger.Args("status", resp.Status))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch file from %s: %s", url, resp.Status)
	}

	tmpFile, err := os.CreateTemp("", "uvbox-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}

	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return tmpFile.Name(), nil
}

func extractWheelEntryInDirectory(directory, entryName string) (string, error) {
	// Read the embedded wheel file
	wheelFile, err := wheelFiles.Open("wheels/" + entryName)
	if err != nil {
		return "", fmt.Errorf("failed to open embedded wheel %s: %w", entryName, err)
	}
	defer wheelFile.Close()

	// Create the destination file
	destFilePath := filepath.Join(directory, entryName)
	destFile, err := os.Create(destFilePath)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file %s: %w", entryName, err)
	}
	defer destFile.Close()

	// Copy the content of the wheel file to the destination file
	_, err = io.Copy(destFile, wheelFile)
	if err != nil {
		return "", fmt.Errorf("failed to copy embedded wheel %s: %w", entryName, err)
	}

	return destFilePath, nil
}
