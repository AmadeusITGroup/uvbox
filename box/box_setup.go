package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mholt/archives"
	"github.com/theckman/yacspin"
)

var CONFIGURATION_FILENAME = "uvbox.toml"
var CERTIFICATES_BUNDLE_FILENAME = "ca-bundle.crt"

// NeedsInstall returns true if the box needs to be installed
func (b *Box) NeedsInstall() (bool, error) {
	if yes, err := b.uvInstalled(); err != nil {
		return false, err
	} else if !yes {
		return true, nil
	}

	if yes, err := b.uvboxConfigurationInstalled(); err != nil {
		return false, err
	} else if !yes {
		return true, nil
	}

	return false, nil
}

// InstallWithSpinner installs the box with a spinner showing progress to the user
func (b *Box) InstallWithSpinner() error {
	cfg := yacspin.Config{
		Frequency: 50 * time.Millisecond,
		CharSet:   yacspin.CharSets[14],
		// Message with wrench emoji
		Suffix:          " 📦 Preparing environment...",
		SuffixAutoColon: true,
	}
	spinner, _ := yacspin.New(cfg)

	spinnerErr := spinner.Start()
	if spinnerErr != nil {
		fmt.Printf("Failed to start spinner: %v\n", spinnerErr)
	}

	err := b.Install()
	if err != nil {
		spinner.StopFailMessage("Failed to set up environment")
		spinnerErr = spinner.StopFail()
		if spinnerErr != nil {
			fmt.Printf("Failed to stop spinner: %v\n", spinnerErr)
		}
		return err
	} else {
		if spinnerErr == nil {
			spinnerErr = spinner.Stop()
			if spinnerErr != nil {
				fmt.Printf("Failed to stop spinner: %v\n", spinnerErr)
			}
		}
	}
	return nil
}

// Install installs the box
// It installs Python, uv, uv configuration, uvbox configuration and launcher script
// If any of these components are already installed, they are skipped
func (b *Box) Install() error {
	if err := b.installUvIfNeeded(); err != nil {
		return fmt.Errorf("error while installing uv if needed: %w", err)
	}

	if err := b.installUvboxConfigurationIfNeeded(); err != nil {
		return fmt.Errorf("error while installing configuration file if needed: %w", err)
	}

	if err := b.installCertificatesBundleIfNeeded(); err != nil {
		return fmt.Errorf("error while installing certificates bundle file if needed: %w", err)
	}

	return nil
}

func (b *Box) Uninstall() error {
	logger.Trace("Uninstalling box")

	if err := deleteFolderWithLogs(b.dedicatedFolder()); err != nil {
		return fmt.Errorf("error while deleting folder: %w", err)
	}

	return nil
}

func (b *Box) CleanCache() error {
	if err := b.uvCacheClean(); err != nil {
		return fmt.Errorf("error while cleaning uv cache: %w", err)
	}

	return nil
}

// installUvIfNeeded installs uv if it is not already installed
func (b *Box) installUvIfNeeded() error {
	if yes, err := b.uvInstalled(); err != nil {
		return fmt.Errorf("error while checking if uv is installed: %w", err)
	} else if !yes {
		if err := b.installUv(); err != nil {
			return fmt.Errorf("error while installing uv: %w", err)
		}
	}

	return nil
}

// installUvboxConfigurationIfNeeded installs uvbox configuration if it is not already installed
func (b *Box) installUvboxConfigurationIfNeeded() error {
	if yes, err := b.uvboxConfigurationInstalled(); err != nil {
		return fmt.Errorf("error while checking if configuration file is installed: %w", err)
	} else if !yes {
		if err := b.installUvboxConfiguration(); err != nil {
			return fmt.Errorf("error while installing configuration file: %w", err)
		}
	}

	return nil
}

func (b *Box) installCertificatesBundleIfNeeded() error {
	if yes, err := b.certificatesBundleInstalled(); err != nil {
		return fmt.Errorf("error while checking if certificates bundle file is installed: %w", err)
	} else if !yes {
		if err := b.installCertificatesBundle(); err != nil {
			return fmt.Errorf("error while installing certificates bundle file: %w", err)
		}
	}

	return nil
}

// installUv installs uv
func (b *Box) installUv() error {
	if err := b.createUvDirectory(); err != nil {
		return err
	}

	if err := b.installDownloadedUv(); err != nil {
		return err
	}

	// Avoid to be disturbed by uv folder name which depends on the platform
	if err := b.symlinkUvExecutable(); err != nil {
		return err
	}

	return nil
}

// installUvboxConfiguration installs uvbox configuration
func (b *Box) installUvboxConfiguration() error {
	if err := b.createDedicatedConfigurationFolder(); err != nil {
		return fmt.Errorf("error while trying to create configuration file directory: %w", err)
	}

	if err := b.installEmbeddedUvboxConfiguration(); err != nil {
		return fmt.Errorf("error while installing embedded configuration file: %w", err)
	}

	return nil
}

func (b *Box) installCertificatesBundle() error {
	if err := b.createDedicatedConfigurationFolder(); err != nil {
		return fmt.Errorf("error while trying to create configuration file directory: %w", err)
	}

	if err := b.installEmbeddedCertificatesBundle(); err != nil {
		return fmt.Errorf("error while installing embedded certificates bundle file: %w", err)
	}

	return nil
}

// uvInstalled returns true if uv is installed
func (b *Box) uvInstalled() (bool, error) {
	logger.Trace("Checking if uv is installed")

	executable, err := b.InstalledUvExecutablePath()
	if err != nil {
		return false, err
	}
	_, err = os.Stat(executable)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	return true, nil
}

// uvboxConfigurationInstalled returns true if uvbox configuration is installed
func (b *Box) uvboxConfigurationInstalled() (bool, error) {
	logger.Trace("Checking if configuration file is installed")

	configFile, err := b.InstalledUvboxConfigurationFilePath()
	if err != nil {
		return false, fmt.Errorf("error while getting configuration file path: %w", err)
	}

	_, err = os.Stat(configFile)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("error while getting configuration file metadata: %w", err)
	}

	return true, nil
}

func (b *Box) certificatesBundleInstalled() (bool, error) {
	logger.Trace("Checking if certificates bundle file is installed")

	bundleFile, err := b.InstalledCertificatesBundleFilePath()
	if err != nil {
		return false, fmt.Errorf("error while getting certificates bundle file path: %w", err)
	}

	_, err = os.Stat(bundleFile)
	if os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, fmt.Errorf("error while getting certificates bundle file metadata: %w", err)
	}

	return true, nil
}

// createUvDirectory creates the directory where uv will be installed
func (b *Box) createUvDirectory() error {
	uvFolder := b.dedicatedUvFolderPath()

	err := os.MkdirAll(uvFolder, 0755)
	if err != nil {
		return err
	}

	return nil
}

func (b *Box) createDedicatedConfigurationFolder() error {
	configFolder := b.dedicatedConfigurationFolderPath()

	err := os.MkdirAll(configFolder, 0755)
	if err != nil {
		return err
	}

	return nil
}

func deleteFolderWithLogs(path string) error {
	logger.Trace("Deleting folder", logger.Args("path", path))
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("failed to delete folder %s: %w", path, err)
	}

	return nil
}

// installDownloadedUv extracts the embedded uv archive to the uv directory
func (b *Box) installDownloadedUv() error {
	uvFolder := b.dedicatedUvFolderPath()
	uvArchive, err := b.downloadUv()
	if err != nil {
		return fmt.Errorf("failed to download uv: %w", err)
	}
	return extractArchive(uvArchive, uvFolder)
}

// symlinkUvExecutable creates a symlink to the uv executable at the root of the uv folder
func (b *Box) symlinkUvExecutable() error {
	uvFolder := b.dedicatedUvFolderPath()
	return symlinkExecutableAtRoot("uv", uvFolder)
}

// installEmbeddedUvboxConfiguration extracts the embedded uvbox configuration file to the configuration folder
func (b *Box) installEmbeddedUvboxConfiguration() error {
	configFolder := b.dedicatedConfigurationFolderPath()
	destination := filepath.Join(configFolder, CONFIGURATION_FILENAME)

	if err := os.WriteFile(destination, embeddedConfigurationFile, 0644); err != nil {
		return fmt.Errorf("error while writing %s: %w", destination, err)
	}

	return nil
}

func (b *Box) installEmbeddedCertificatesBundle() error {
	configFolder := b.dedicatedConfigurationFolderPath()
	destination := filepath.Join(configFolder, CERTIFICATES_BUNDLE_FILENAME)
	if err := os.WriteFile(destination, embeddedCertificatesBundle, 0644); err != nil {
		return fmt.Errorf("error while writing %s: %w", destination, err)
	}

	return nil
}

// symlinkExecutableAtRoot creates a symlink of the searched executable name if found, at the root of the destination folder
func symlinkExecutableAtRoot(executableName string, destination string) error {
	unixExecutableName := strings.TrimSuffix(executableName, filepath.Ext(executableName))
	windowsExecutableName := unixExecutableName + ".exe"
	// Move uv and uvx to their parent parent folder
	symlinkExecutables := func(path string, d os.DirEntry, err error) error {
		if d.Type().IsRegular() {
			if d.Name() == unixExecutableName || d.Name() == windowsExecutableName {
				newPath := filepath.Join(destination, d.Name())
				logger.Trace("Moving", logger.Args("name", d.Name(), "from", path, "to", newPath))
				if err := os.Rename(path, newPath); err != nil {
					return fmt.Errorf("failed to move %s to %s: %w", path, newPath, err)
				}
				return nil
			}
		}
		return nil
	}
	if err := filepath.WalkDir(destination, symlinkExecutables); err != nil {
		return err
	}

	// Check if symlink was created
	_, unixErr := os.Stat(filepath.Join(destination, unixExecutableName))
	_, windowsErr := os.Stat(filepath.Join(destination, windowsExecutableName))
	if unixErr != nil && windowsErr != nil {
		return fmt.Errorf("uv executable should be present and valid at: %s", executableName)
	}

	return nil
}

// extractArchive extracts an archive (zip or tar.gz) to a destination folder.
// It uses mholt/archives which auto-detects the archive format from the content.
func extractArchive(source []byte, destination string) error {
	ctx := context.Background()
	reader := bytes.NewReader(source)

	// Auto-detect archive format (zip, tar.gz, etc.) by peeking at the stream header
	format, stream, err := archives.Identify(ctx, "", reader)
	if err != nil {
		return fmt.Errorf("failed to identify archive format: %w", err)
	}

	// Ensure the detected format supports extraction
	extractor, ok := format.(archives.Extractor)
	if !ok {
		return fmt.Errorf("format does not support extraction")
	}

	// Normalize destination to absolute path for secure path validation
	destination, err = filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("failed to get absolute destination path: %w", err)
	}

	// Create the destination directory if it doesn't exist
	if err := os.MkdirAll(destination, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Handler is called for each file in the archive.
	// The library uses this callback pattern to give us control over how files are written,
	// allowing us to implement security checks and handle different file types appropriately.
	handler := func(ctx context.Context, f archives.FileInfo) error {
		targetPath := filepath.Join(destination, f.NameInArchive)

		// Security: Prevent path traversal attacks (e.g., "../../../etc/passwd")
		// by ensuring the resolved path stays within our destination directory
		cleanPath := filepath.Clean(targetPath)
		if !strings.HasPrefix(cleanPath, filepath.Clean(destination)+string(os.PathSeparator)) {
			return fmt.Errorf("invalid file path (path traversal attempt): %s", f.NameInArchive)
		}

		// Handle directories: create them and continue to next entry
		if f.IsDir() {
			return os.MkdirAll(targetPath, f.Mode())
		}

		// Ensure parent directories exist for this file
		if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
			return fmt.Errorf("failed to create parent directory for %s: %w", targetPath, err)
		}

		// Handle symbolic links
		if f.LinkTarget != "" {
			return os.Symlink(f.LinkTarget, targetPath)
		}

		// Handle regular files: open from archive, create on disk, copy contents
		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("failed to open file in archive: %w", err)
		}
		defer rc.Close()

		outFile, err := os.OpenFile(targetPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return fmt.Errorf("failed to create file %s: %w", targetPath, err)
		}
		defer outFile.Close()

		if _, err := io.Copy(outFile, rc); err != nil {
			return fmt.Errorf("failed to write file contents to %s: %w", targetPath, err)
		}

		// Preserve original modification time from the archive
		if err := os.Chtimes(targetPath, f.ModTime(), f.ModTime()); err != nil {
			return fmt.Errorf("failed to set modification time for %s: %w", targetPath, err)
		}

		return nil
	}

	// Walk through all entries in the archive and process each one via the handler
	if err := extractor.Extract(ctx, stream, handler); err != nil {
		return fmt.Errorf("failed to extract archive: %w", err)
	}

	return nil
}

func (b *Box) determineUvDownloadUrl() (string, error) {
	baseReleasesUrl := "https://github.com/astral-sh/uv/releases/download"
	baseVersion := "0.4.20"

	releasesUrl := ""
	version := ""

	if b.UvInstallationReleasesMirror != "" {
		releasesUrl = b.UvInstallationReleasesMirror
		releasesUrl = strings.TrimSuffix(releasesUrl, "/")
	} else {
		releasesUrl = baseReleasesUrl
	}

	if b.UvVersion != "" {
		version = b.UvVersion
	} else {
		version = baseVersion
	}

	uvUrls := map[string]string{
		"darwin/amd64":  fmt.Sprintf("%s/%s/uv-x86_64-apple-darwin.tar.gz", releasesUrl, version),
		"darwin/arm64":  fmt.Sprintf("%s/%s/uv-aarch64-apple-darwin.tar.gz", releasesUrl, version),
		"linux/amd64":   fmt.Sprintf("%s/%s/uv-x86_64-unknown-linux-gnu.tar.gz", releasesUrl, version),
		"linux/arm64":   fmt.Sprintf("%s/%s/uv-aarch64-unknown-linux-gnu.tar.gz", releasesUrl, version),
		"windows/amd64": fmt.Sprintf("%s/%s/uv-x86_64-pc-windows-msvc.zip", releasesUrl, version),
		"windows/arm64": fmt.Sprintf("%s/%s/uv-aarch64-pc-windows-msvc.zip", releasesUrl, version),
	}

	target := fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)

	url, ok := uvUrls[target]
	if !ok {
		return "", fmt.Errorf("failed to determine uv download url for %s", target)
	}

	return url, nil

}

func downloadArchiveContent(url string) ([]byte, error) {
	// Download the file
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to create GET HTTP request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to perform get request: %w", err)
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return content, fmt.Errorf("failed to read response content: %w", err)
	}

	return content, nil
}

func (b *Box) downloadUv() ([]byte, error) {
	url, err := b.determineUvDownloadUrl()
	if err != nil {
		return []byte{}, fmt.Errorf("failed to determine uv download url: %w", err)
	}

	logger.Debug("Downloading UV", logger.Args("URL", url))

	content, err := downloadArchiveContent(url)
	if err != nil {
		return []byte{}, fmt.Errorf("failed to download uv: %w", err)
	}

	logger.Trace("Downloaded UV", logger.Args("URL", url, "Length", len(content)))

	return content, nil
}
