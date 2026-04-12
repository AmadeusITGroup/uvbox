package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateUvHomeFolder(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	installedTextBoxWithDummyPackage(t, homePath, "myIdentifier")

	expectedPath := filepath.Join(homePath, "uvbox", "myIdentifier", "uv")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Fatalf("expected folder %s to exist", expectedPath)
	} else if err != nil {
		t.Fatalf("could not check if folder %s exists: %v", expectedPath, err)
	}
}

func TestDoesNotRemoveExistingUvFolderIfAlreadyInstalled(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	box := installedTextBoxDummy(t, homePath)

	// Create file in uv folder
	fileWhichShouldNotBeRemoved := filepath.Join(box.dedicatedUvFolderPath(), "myfile")
	err := os.WriteFile(fileWhichShouldNotBeRemoved, []byte("hello"), 0644)
	if err != nil {
		t.Fatalf("could not create file %s: %v", fileWhichShouldNotBeRemoved, err)
	}
	// Assert file exists
	if _, err := os.Stat(fileWhichShouldNotBeRemoved); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist", fileWhichShouldNotBeRemoved)
	} else if err != nil {
		t.Fatalf("could not check if file %s exists: %v", fileWhichShouldNotBeRemoved, err)
	}

	// Reinstall
	err = box.Install()
	if err != nil {
		t.Fatalf("could not reinstall: %v", err)
	}

	// Assert file still exists
	if _, err := os.Stat(fileWhichShouldNotBeRemoved); os.IsNotExist(err) {
		t.Fatalf("expected file %s to exist", fileWhichShouldNotBeRemoved)
	} else if err != nil {
		t.Fatalf("could not check if file %s exists: %v", fileWhichShouldNotBeRemoved, err)
	}
}

func TestInstallUvBinaries(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	box := installedTextBoxDummy(t, homePath)

	// Check if uv binary exists
	executable, err := box.InstalledUvExecutablePath()
	if err != nil {
		t.Fatalf("could not get installed uv executable path: %v", err)
	}
	if _, err := os.Stat(executable); os.IsNotExist(err) {
		t.Fatalf("expected uv binary %s to exist", executable)
	} else if err != nil {
		t.Fatalf("could not check if uv binary %s exists: %v", executable, err)
	}
}

func TestUvExecutableIsUnderUvFolder(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	box := installedTextBoxWithDummyPackage(t, homePath, "myIdentifier")

	executable, err := box.InstalledUvExecutablePath()
	if err != nil {
		t.Fatalf("could not get installed uv executable path: %v", err)
	}

	expectedParent := filepath.Join(homePath, "uvbox", "myIdentifier", "uv")
	_, err = filepath.Rel(expectedParent, executable)
	if err != nil {
		t.Fatalf("could not get relative path: %v", err)
	}
}

func TestUvExecutableIsCallable(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	box := installedTextBoxDummy(t, homePath)

	executable, err := box.InstalledUvExecutablePath()
	if err != nil {
		t.Fatalf("could not get installed uv executable path: %v", err)
	}

	// Run uv
	out, err := exec.Command(executable, "--version").Output()
	if err != nil {
		t.Fatalf("could not run uv: %v", err)
	}

	version := string(out)
	strings.Contains(version, "uv")
}

func TestCanCleanUvCache(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	box := installedTextBoxWithPackage(t, homePath, "cowsay", "6.1", "")

	cachePath := box.uvCacheDirPath()
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		logger.Fatal("expected cache folder to exists after package installation", logger.Args("package", box.PackageName, "path", cachePath))
	} else if err != nil {
		logger.Fatal("could not check if cache folder exists", logger.Args("package", box.PackageName, "path", cachePath, "error", err))
	}

	err := box.CleanCache()
	if err != nil {
		logger.Fatal("could not clear cache", logger.Args("package", box.PackageName, "error", err))
	}

	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		logger.Fatal("expected cache folder to be removed after cleaning", logger.Args("package", box.PackageName, "path", cachePath))
	} else if err != nil && !os.IsNotExist(err) {
		logger.Fatal("could not check if cache folder exists", logger.Args("package", box.PackageName, "path", cachePath, "error", err))
	}
}

func TestCanInvokeManualUvCommand(t *testing.T) {
	homePath := testDirectory(t)
	defer os.RemoveAll(homePath)

	box := installedTextBoxWithPackage(t, homePath, "cowsay", "6.1", "")

	command := []string{"--version"}
	returnCode, err := box.RunUv(command)
	if err != nil {
		logger.Fatal("could not run uv command", logger.Args("error", err))
	} else if returnCode != 0 {
		logger.Fatal("expected command to return 0", logger.Args("command", command, "returnCode", returnCode))
	}
}
