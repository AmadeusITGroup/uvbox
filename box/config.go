package main

import (
	"crypto"
	"fmt"
	"io"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Configuration struct {
	Package PackageConfiguration
	Certs   CertificatesConfiguration
	Uv      UvConfiguration
}

type UpdateConfiguration struct {
	Url string
}

type PackageConfiguration struct {
	Name        string
	Script      string
	Version     PackageVersionConfiguration
	Constraints PackageConstraintsConfiguration
}

type PackageVersionConfiguration struct {
	AutoUpdate bool `toml:"auto-update"`
	Static     string
	Dynamic    string
}

type PackageConstraintsConfiguration struct {
	Static  string
	Dynamic string
}

type CertificatesConfiguration struct {
	Path string
}

type UvConfiguration struct {
	Version     string
	Mirror      string
	Environment []string
}

func loadConfiguration() (Configuration, error) {
	config := Configuration{}

	err := toml.Unmarshal(embeddedConfigurationFile, &config)
	if err != nil {
		return config, fmt.Errorf("failed to unmarshal configuration: %v", err)
	}

	return config, nil
}

func (c Configuration) PanicIfInvalid() {
	if c.Package.Name == "" {
		logger.Fatal("configuration file is missing package.name")
	}
	if c.Package.Script == "" {
		logger.Fatal("configuration file is missing package.script")
	}
}

func (c Configuration) ComputeIdentifier() string {
	hasher := crypto.SHA1.New()
	textToHash := fmt.Sprintf("%s%s%s%t", c.Package.Name, c.Package.Script, c.Package.Version.Static, c.AutoUpdateEnabled())
	if GIT_SOURCE != "" {
		textToHash += GIT_SOURCE
	}
	_, err := io.WriteString(hasher, textToHash)
	if err != nil {
		logger.Fatal("Failed to hash script value", logger.Args("error", err))
	}
	identifier := fmt.Sprintf("%s-%x", c.Package.Name, hasher.Sum(nil))
	return identifier
}

func (c Configuration) PackageName() string {
	return c.Package.Name
}

func (c Configuration) PackageScript() string {
	return c.Package.Script
}

func (c Configuration) AutoUpdateEnabled() bool {
	return c.Package.Version.AutoUpdate
}

func (c Configuration) PackageVersion() (string, error) {
	logger.Trace("Checking package version")
	packageVersionWanted := c.Package.Version.Static
	if c.Package.Version.Static == "" {
		if c.Package.Version.Dynamic == "" {
			return "", nil
		}

		logger.Trace("Reading dynamic version")
		if value, err := readVersionFromRemote(c.Package.Version.Dynamic); err != nil {
			return "", fmt.Errorf("failed to read dynamic version: %v", err)
		} else {
			logger.Trace("Dynamic version read", logger.Args("version", value))
			packageVersionWanted = value
		}
	}
	return packageVersionWanted, nil
}

func (c Configuration) PackageConstraintsFileUrl(version string) string {
	constraintsUrl := c.Package.Constraints.Static
	if constraintsUrl == "" {
		constraintsUrl = c.Package.Constraints.Dynamic
	}

	if constraintsUrl == "" {
		return ""
	}

	return strings.ReplaceAll(constraintsUrl, "<VERSION>", version)
}

func (c Configuration) CertificatesBundlePath() string {
	return c.Certs.Path
}

func (c Configuration) UvVersion() string {
	return c.Uv.Version
}

func (c Configuration) UvMirror() string {
	return c.Uv.Mirror
}

func (c Configuration) UvEnviron() []string {
	return c.Uv.Environment
}
