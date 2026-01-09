package main

//go:generate go run generate.go

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed uvbox.toml
var embeddedConfigurationFile []byte

//go:embed ca-bundle.crt
var embeddedCertificatesBundle []byte

// Box represents a uvbox instance.
// It is identified by an installation identifier and has a home path, which would preferally be XDG_DATA_HOME.
// A box can install and run python packages.
// It also allows to install and access everything needed for a box installation:
//
// - Python
//
// - uv
//
// - configuration files
//
// - launcher script
type Box struct {
	InstallationIdentifier       string
	PackageName                  string
	HomePath                     string
	CertificatesBundle           string
	UvExtraEnviron               []string
	UvInstallationReleasesMirror string
	UvVersion                    string
}

// GetBox returns a new Box instance with the given identifier and home path.
func GetBox(identifier, packageName string, uvExtraEnviron []string, homePath string) *Box {
	box := &Box{
		InstallationIdentifier:       identifier,
		PackageName:                  packageName,
		HomePath:                     homePath,
		CertificatesBundle:           "",
		UvExtraEnviron:               uvExtraEnviron,
		UvInstallationReleasesMirror: "",
		UvVersion:                    "",
	}

	box.logUvEnvironmentVariables()
	return box
}

// SetCertificatesBundlePath set the certificates ca bundle that will be used for SSL_CERT_FILE and REQUESTS_CA_BUNDLE environment variables
func (b *Box) SetCertificatesBundlePath(path string) {
	b.CertificatesBundle = path
}

func (b *Box) SetUvInstallationReleasesMirror(url string) {
	b.UvInstallationReleasesMirror = url
}

func (b *Box) SetUvVersion(version string) {
	b.UvVersion = version
}

func (b *Box) uvEnvironmentVariables() []string {
	// Add uv configuration environment variables
	boxUvBaseConfiguration := []string{
		"UV_NO_CONFIG=1",
		fmt.Sprintf("UV_CACHE_DIR=%s", b.uvCacheDirPath()),
		fmt.Sprintf("UV_PYTHON_INSTALL_DIR=%s", b.uvPythonDirPath()),
		fmt.Sprintf("UV_TOOL_DIR=%s", b.uvToolDirPath()),
		fmt.Sprintf("UV_TOOL_BIN_DIR=%s", b.uvToolBinDirPath()),
	}
	// Get extra uv environment variables coming from the user configuration file
	userExtraUvEnvironment := b.UvExtraEnviron

	// Merge configurations
	env := []string{}
	env = append(env, boxUvBaseConfiguration...)
	env = append(env, userExtraUvEnvironment...)

	return env
}

func (b *Box) logUvEnvironmentVariables() {
	logger.Debug("Using UV environment variables", logger.ArgsFromMap(environListAsMap(b.uvEnvironmentVariables())))
}

func (b *Box) commandsEnvironment() ([]string, error) {
	// Get uv environment variables for this box
	env := b.uvEnvironmentVariables()

	// Add current user environment variables, filtering out any Python-related
	// environment variables that could cause conflicts when a uvbox binary
	// runs another uvbox binary
	userEnv, filteredKeys := filterOutPythonEnvironmentVariables(os.Environ())
	if len(filteredKeys) > 0 {
		logger.Debug("Filtered out inherited Python environment variables", logger.ArgsFromMap(map[string]any{"filteredKeys": filteredKeys}))
	}
	env = append(env, userEnv...)

	// Add SSL_CERT_FILE and REQUESTS_CA_BUNDLE if asked by the user
	if b.CertificatesBundle != "" {
		bundlePath, err := b.InstalledCertificatesBundleFilePath()
		if err != nil {
			return []string{}, fmt.Errorf("could not get certificates bundle file path: %w", err)
		}

		env = append(
			env,
			fmt.Sprintf("SSL_CERT_FILE=%s", bundlePath),
			fmt.Sprintf("REQUESTS_CA_BUNDLE=%s", bundlePath),
		)
	}

	return env, nil
}

// InstalledUvExecutablePath returns the path to the uv executable for the current installation.
// It does not check if the file exists.
func (b *Box) InstalledUvExecutablePath() (string, error) {
	uvFolder := b.dedicatedUvFolderPath()

	if runtime.GOOS == "windows" {
		return filepath.Join(uvFolder, "uv.exe"), nil
	} else {
		return filepath.Join(uvFolder, "uv"), nil
	}
}

// InstalledUvboxConfigurationFilePath returns the path to the uvbox configuration file for the current installation.
// It does not check if the file exists.
func (b *Box) InstalledUvboxConfigurationFilePath() (string, error) {
	return filepath.Join(b.dedicatedConfigurationFolderPath(), CONFIGURATION_FILENAME), nil
}

func (b *Box) InstalledCertificatesBundleFilePath() (string, error) {
	return filepath.Join(b.dedicatedConfigurationFolderPath(), CERTIFICATES_BUNDLE_FILENAME), nil
}

// dedicatedConfigurationFolderPath returns the path to the configuration folder for the current installation.
func (b *Box) dedicatedConfigurationFolderPath() string {
	return b.dedicatedSubfolder("configuration")
}

// dedicatedUvFolderPath returns the path to the uv installation folder for the current installation.
func (b *Box) dedicatedUvFolderPath() string {
	return b.dedicatedSubfolder("uv")
}

func (b *Box) uvToolDirPath() string {
	return b.dedicatedSubfolder("tools")
}

func (b *Box) uvToolBinDirPath() string {
	return b.dedicatedSubfolder("tools-bin")
}

func (b *Box) uvPythonDirPath() string {
	return b.dedicatedSubfolder("python")
}

func (b *Box) uvCacheDirPath() string {
	return b.dedicatedSubfolder("cache")
}

func (b *Box) dedicatedSubfolder(folderName string) string {
	return filepath.Join(b.dedicatedFolder(), folderName)
}

func (b *Box) dedicatedFolder() string {
	return filepath.Join(b.uvboxHomePath(), b.InstallationIdentifier)
}

// uvboxHomePath returns the path to the uvbox home path.
func (b *Box) uvboxHomePath() string {
	return filepath.Join(b.HomePath, "uvbox")
}

func environListAsMap(environment []string) map[string]any {
	envMap := make(map[string]any)
	for _, env := range environment {
		if i := strings.Index(env, "="); i >= 0 {
			envMap[env[:i]] = env[i+1:]
		}
	}
	return envMap
}

// filterOutPythonEnvironmentVariables removes environment variables that could
// cause conflicts when a uvbox binary runs another uvbox binary.
// Filtered variables:
// - UV_* : uv tool configuration (each uvbox binary has its own)
// - VIRTUAL_ENV : virtual environment path (uvbox uses its own isolated env)
// - PYTHONPATH : Python module search path (uvbox should use its own packages)
func filterOutPythonEnvironmentVariables(env []string) (filtered []string, removed []string) {
	filtered = []string{}
	removed = []string{}
	for _, e := range env {
		key := e
		if idx := strings.Index(e, "="); idx != -1 {
			key = e[:idx]
		}

		if strings.HasPrefix(key, "UV_") || key == "VIRTUAL_ENV" || key == "PYTHONPATH" {
			removed = append(removed, key)
		} else {
			filtered = append(filtered, e)
		}
	}
	return filtered, removed
}
