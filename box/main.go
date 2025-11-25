package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/adrg/xdg"
)

func main() {
	// Load user configuration
	packageName, packageVersion, packageScript, packageConstraintsUrl, identifier, certificatesBundlePath, autoUpdateEnabled, uvVersion, uvMirror, uvEnviron, versionCheckErr := runConfiguration()

	// Instantiate the installation box
	box := GetBox(identifier, packageName, uvEnviron, xdg.DataHome)
	box.SetUvVersion(uvVersion)
	box.SetUvInstallationReleasesMirror(uvMirror)
	box.SetCertificatesBundlePath(certificatesBundlePath)

	// Check if user has run the hidden management CLI
	runHiddenCliAndExitIfCalled(box, packageVersion, packageConstraintsUrl)

	// Install/Update
	ensurePackageUpToDateInstallation(box, packageName, versionCheckErr, packageVersion, packageConstraintsUrl, autoUpdateEnabled)

	// Run
	runPackageAndExit(packageName, packageScript, box)
}

func runConfiguration() (packageName, packageVersion, packageScript, packageConstraintsUrl, identifier, certificatesBundlePath string, autoUpdateEnabled bool, uvVersion, uvMirror string, uvEnviron []string, versionCheckErr error) {
	logger.Trace("Loading binary configuration file")
	cfg, err := loadConfiguration()
	if err != nil {
		logger.Fatal("Failed to load configuration", logger.Args("error", err))
	}
	cfg.PanicIfInvalid()

	packageName = cfg.PackageName()
	packageVersion, versionCheckErr = cfg.PackageVersion()
	packageScript = cfg.PackageScript()
	packageConstraintsUrl = cfg.PackageConstraintsFileUrl(packageVersion)
	identifier = cfg.ComputeIdentifier()
	certificatesBundlePath = cfg.CertificatesBundlePath()
	uvVersion = cfg.UvVersion()
	uvMirror = cfg.UvMirror()
	uvEnviron = cfg.UvEnviron()
	autoUpdateEnabled = cfg.AutoUpdateEnabled()

	logger.Debug("Configuration loaded", logger.Args("package", packageName, "version", packageVersion, "script", packageScript, "constraints", packageConstraintsUrl, "identifier", identifier, "certificates", certificatesBundlePath, "autoUpdate", autoUpdateEnabled, "uv version", uvVersion, "uv mirror", uvMirror, "uv environ", uvEnviron))

	return
}

func runPackageAndExit(packageName string, packageScript string, box *Box) {
	logger.Trace("Running", logger.Args("package", packageName, "script", packageScript))
	// Ignore SIGINT signal to prevent exiting when the package is running, only the package should handle it
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	// Run the package
	returnCode, err := box.Run(packageName, packageScript)
	if err != nil {
		logger.Fatal("Failed to run script", logger.Args("package", packageName, "script", packageScript, "error", err))
	} else {
		logger.Debug("Script executed", logger.Args("package", packageName, "script", packageScript, "returnCode", returnCode))
		os.Exit(returnCode)
	}
}

func ensurePackageUpToDateInstallation(box *Box, packageName string, versionCheckErr error, packageVersion string, packageConstraintsUrl string, autoUpdateEnabled bool) {
	if yes, err := box.NeedsInstall(); err != nil {
		logger.Fatal("Failed to check if box needs install", logger.Args("error", err))
	} else if yes {
		logger.Trace("Installing box")
		if err := box.InstallWithSpinner(); err != nil {
			logger.Fatal("Failed to install box", logger.Args("error", err))
		}
	}

	if yes, err := box.IsPackageInstalled(); err != nil {
		logger.Fatal("Failed to check if already installed", logger.Args("package", packageName, "error", err))
	} else if yes && versionCheckErr != nil {
		logger.Warn("Could not check version", logger.Args("package", packageName, "error", versionCheckErr))
	} else if !yes && versionCheckErr != nil {
		logger.Fatal("Could not check version", logger.Args("package", packageName, "error", versionCheckErr))
	} else if !yes {
		logger.Trace("Not installed", logger.Args("package", packageName))

		if err := box.InstallPackageWithSpinner(packageVersion, packageConstraintsUrl); err != nil {
			logger.Fatal("Failed to install", logger.Args("package", packageName, "version", packageVersion, "error", err))
		}
	} else {
		logger.Trace("Installed", logger.Args("package", packageName))

		if autoUpdateEnabled {
			logger.Trace("Checking for updates")

			currentPackageVersion, err := box.InstalledPackageVersion()
			if err != nil {
				logger.Fatal("Failed to get installed version", logger.Args("package", packageName, "error", err))
			}

			if currentPackageVersion != packageVersion {
				logger.Debug("Package is installed but version is different", logger.Args("package", packageName, "currentVersion", currentPackageVersion, "wantedVersion", packageVersion))

				if err := box.UpdatePackageWithSpinner(currentPackageVersion, packageVersion, packageConstraintsUrl); err != nil {
					logger.Error("Failed to update", logger.Args("package", packageName, "currentVersion", currentPackageVersion, "newVersion", packageVersion, "error", err))
				}
			}
		}
	}
}

func trimReaderContent(reader io.Reader) (string, error) {
	content, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return trimText(string(content)), nil
}

func trimText(text string) string {
	firstLine := strings.Split(text, "\n")[0]
	return strings.Trim(firstLine, "\n\r\t\b ")
}

func readVersionFromRemote(url string) (string, error) {
	logger.Debug("Fetching version", logger.Args("url", url))
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("failed to fetch version from %s: %v", url, err)
	}
	defer resp.Body.Close()

	logger.Debug("Fetched", logger.Args("status", resp.Status))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch version from %s: %s", url, resp.Status)
	}
	return trimReaderContent(resp.Body)
}
