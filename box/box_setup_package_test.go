package main

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/h2non/gock"
)

func installedTextBoxWithPackage(t *testing.T, homePath, packageName, packageVersion, packageConstraintsUrl string) *Box {
	box := GetBox("myIdentifier", packageName, []string{}, homePath)

	err := box.Install()
	if err != nil {
		t.Fatalf("Error while installing box: %v", err)
	}

	err = box.InstallPackage(packageVersion, packageConstraintsUrl)
	if err != nil {
		t.Fatalf("Error while installing package ''%s==%s' with constraints url '%s'", packageName, packageVersion, packageConstraintsUrl)
	}

	return box
}

func TestCanInstallPackageWithSpinner(t *testing.T) {
	hideStdout()

	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)
	box := installedTestBox(t, homePath, "myIdentifier", "cowsay")

	packageVersion := "6.1"

	err := box.InstallPackageWithSpinner(packageVersion, "")
	if err != nil {
		t.Fatalf("could not install package %s: %v", box.PackageName, err)
	}

	if yes, err := box.IsPackageInstalled(); err != nil {
		t.Fatalf("could not check if package %s is installed: %v", box.PackageName, err)
	} else if !yes {
		t.Fatalf("expected package %s to be installed", box.PackageName)
	}
}

func TestCanUpdatePackageWithSpinner(t *testing.T) {
	hideStdoutAndStderr()

	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	packageToInstall := "cowsay"
	initialPackageVersion := "6.0"
	box := installedTextBoxWithPackage(t, homePath, packageToInstall, initialPackageVersion, "")

	newPackageVersion := "6.1"
	err := box.UpdatePackageWithSpinner(initialPackageVersion, newPackageVersion, "")
	if err != nil {
		t.Fatalf("could not update package %s: %v", packageToInstall, err)
	}

	version, err := box.InstalledPackageVersion()
	if err != nil {
		t.Fatalf("could not get installed version of package %s: %v", packageToInstall, err)
	}

	if version != newPackageVersion {
		t.Fatalf("expected package %s to be version %s, got %s", packageToInstall, newPackageVersion, version)
	}
}

func TestCanInstallPackage(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	packageToInstall := "cowsay"
	packageVersion := "6.1"

	box := installedTextBoxWithPackage(t, homePath, packageToInstall, packageVersion, "")

	if yes, err := box.IsPackageInstalled(); err != nil {
		t.Fatalf("could not check if package %s is installed: %v", packageToInstall, err)
	} else if !yes {
		t.Fatalf("expected package %s to be installed", packageToInstall)
	}
}

func TestCanDetectNotInstalledPackage(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)
	box := installedTestBox(t, homePath, "myIdentifier", "cowsay")

	if yes, err := box.IsPackageInstalled(); err != nil {
		t.Fatalf("could not check if package %s is installed: %v", box.PackageName, err)
	} else if yes {
		t.Fatalf("expected package %s to not be installed", box.PackageName)
	}
}

func TestCanDetectInstalledPackageVersion(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	packageToInstall := "cowsay"
	packageVersion := "6.1"

	box := installedTextBoxWithPackage(t, homePath, packageToInstall, packageVersion, "")

	version, err := box.InstalledPackageVersion()
	if err != nil {
		t.Fatalf("could not get installed version of package %s: %v", packageToInstall, err)
	}

	if version != packageVersion {
		t.Fatalf("expected package %s to be version %s, got %s", packageToInstall, packageVersion, version)
	}
}

func TestCanInstallPytestWithConstraintsPackagingTo240(t *testing.T) {
	defer gock.Off()
	defer gock.DisableNetworking()
	defer gock.DisableNetworkingFilters()

	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	packageToInstall := "pytest"
	packageVersion := "8.3.3"

	gock.EnableNetworking()
	gock.NetworkingFilter(func(req *http.Request) bool {
		return req.URL.Host != "fakeconstraints.example"
	})
	gock.New("http://fakeconstraints.example").
		Get("/constraints.txt").
		Reply(200).
		BodyString("packaging==24.0")

	box := installedTextBoxWithPackage(t, homePath, packageToInstall, packageVersion, "http://fakeconstraints.example/constraints.txt")

	path, err := box.InstalledPackagePath()
	if err != nil {
		t.Fatalf("could not get installation path of package %s: %v", packageToInstall, err)
	}

	uvExecutable, err := box.InstalledUvExecutablePath()
	if err != nil {
		t.Fatalf("could not get installed uv executable path: %v", err)
	}

	cmd := exec.Command(uvExecutable, "pip", "freeze")
	cmd.Dir = path
	cmd.Env = []string{}

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("could not run pip freeze: %v", err)
	}

	expectedFreeze := "packaging==24.0"
	if !strings.Contains(string(out), expectedFreeze) {
		log.Fatalf("expected package %s to be installed, freeze list: %s", expectedFreeze, string(out))
		logger.Fatal("expected package to be installed with correct dependency version", logger.Args("package", packageToInstall, "expected freeze", expectedFreeze, "current freeze", string(out)))
	}
}

func TestCanDetectNotInstalledPackageVersion(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)
	box := installedTextBoxDummy(t, homePath)

	_, err := box.InstalledPackageVersion()
	if err == nil {
		logger.Fatal("expected error when checking version of package %s", logger.Args("package", box.PackageName))
	}
}

func TestCanInstallPackageWithoutSpecifyingVersion(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	packageVersion := ""
	expectedInstalledVersion := "6.1"
	box := installedTextBoxWithPackage(t, homePath, "cowsay", packageVersion, "")

	installedVersion, err := box.InstalledPackageVersion()
	if err != nil {
		logger.Fatal("could not get installed version of package", logger.Args("package", box.PackageName))
	} else if installedVersion != expectedInstalledVersion {
		logger.Fatal("expected package to be installed with latest version", logger.Args("package", box.PackageName, "expected version", expectedInstalledVersion, "installed version", installedVersion))
	}
}
