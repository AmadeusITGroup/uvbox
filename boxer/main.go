package main

//go:generate go run -tags "!rar" generate.go

import (
	"archive/zip"
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/charmbracelet/fang"
	"github.com/mholt/archives"
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"github.com/spf13/cobra"
)

var (
	version = ""
)

var NoBanner bool
var Config string
var Output string
var Nfpm string
var ReleaseVersion string

var Darwin bool
var Linux bool
var Windows bool

var Amd bool
var Arm bool

var PLATFORM_LINUX = "linux"
var PLATFORM_DARWIN = "darwin"
var PLATFORM_WINDOWS = "windows"

var ARCH_AMD64 = "amd64"
var ARCH_ARM64 = "arm64"

var WheelsToEmbed = []string{}

//go:embed embedded_box.tar.gz
var embeddedBoxRepository []byte

func main() {
	// UVBOX PYPI
	var pypiCmd = &cobra.Command{
		Use:   "pypi",
		Short: "Use a pypi package to generate a standalone executable",
		Args:  cobra.NoArgs,
		PreRun: func(cmd *cobra.Command, args []string) {
			preRun()
		},
		Run: func(cmd *cobra.Command, args []string) {
			if err := run(); err != nil {
				logger.Fatal("failed ton run pypi command", logger.Args("error", err))
			}
		},
	}
	pypiCmd.Flags().StringVarP(
		&Config, "config", "c", "", "Configuration file",
	)
	pypiCmd.Flags().StringVarP(
		&Output, "output", "o", "dist", "Output directory",
	)

	pypiCmd.Flags().BoolVarP(&Darwin, "darwin", "d", false, "Build for darwin")
	pypiCmd.Flags().BoolVarP(&Linux, "linux", "l", false, "Build for linux")
	pypiCmd.Flags().BoolVarP(&Windows, "windows", "w", false, "Build for windows")
	pypiCmd.Flags().BoolVarP(&Amd, "amd", "", false, "Build for AMD64")
	pypiCmd.Flags().BoolVarP(&Arm, "arm", "", false, "build for ARM64")

	// UVBOX WHEEL
	var wheelCmd = &cobra.Command{
		Use:   "wheel",
		Short: "Use wheel package(s) to generate a standalone executable",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			WheelsToEmbed = args
			if err := run(); err != nil {
				logger.Fatal("failed to run wheel command", logger.Args("error", err))
			}
		},
	}

	wheelCmd.Flags().StringVarP(
		&Config, "config", "c", "", "Configuration file",
	)
	wheelCmd.Flags().StringVarP(
		&Output, "output", "o", "dist", "Output directory",
	)
	wheelCmd.Flags().BoolVarP(&Darwin, "darwin", "d", false, "Build for darwin")
	wheelCmd.Flags().BoolVarP(&Linux, "linux", "l", false, "Build for linux")
	wheelCmd.Flags().BoolVarP(&Windows, "windows", "w", false, "Build for windows")
	wheelCmd.Flags().BoolVarP(&Amd, "amd", "", false, "Build for AMD64")
	wheelCmd.Flags().BoolVarP(&Arm, "arm", "", false, "build for ARM64")

	// UVBOX
	var rootCmd = &cobra.Command{
		Use:   "uvbox",
		Short: "Generate standalone python executables for darwin, linux and windows!",
		Args:  cobra.NoArgs,
	}
	rootCmd.AddCommand(pypiCmd)
	rootCmd.AddCommand(wheelCmd)
	rootCmd.PersistentFlags().StringVarP(&ReleaseVersion, "release-version", "", "0.0.0", "Specify a version for the binaries. Will be used for example for versionning linux packages.")
	rootCmd.PersistentFlags().BoolVarP(&NoBanner, "no-banner", "", false, "Do not display the banner")
	rootCmd.PersistentFlags().StringVarP(&Nfpm, "nfpm", "", "", "Generate linux packages with the given nfpm configuration file")

	err := fang.Execute(context.Background(), rootCmd, fang.WithVersion(version))
	if err != nil {
		os.Exit(1)
	}
}

func preRun() {
	validateOutputDirectoryFlag()
	validateGoAvailability()
	validateNfpmAvailability()
	validateWheelsToEmbed()
}

func validateOutputDirectoryFlag() {
	if Output == "" {
		logger.Fatal("output directory is required")
	}
}

func validateGoAvailability() {
	_, err := exec.LookPath("go")
	if err != nil {
		logger.Fatal("go is required to build boxes")
	}
}

func validateNfpmAvailability() {
	if Nfpm != "" {
		_, err := exec.LookPath("nfpm")
		if err != nil {
			logger.Fatal("nfpm is required to build linux packages!")
		}
	}
}

func validateWheelsToEmbed() {
	for _, wheel := range WheelsToEmbed {
		if ok, err := fileExists(wheel); err != nil {
			logger.Fatal("error while checking if wheel exists", logger.Args("path", wheel, "error", err))
		} else if !ok {
			logger.Fatal("wheel should be an existing file", logger.Args("path", wheel))
		}
	}
}

type CompilationConfiguration struct {
	OS              string
	ARCH            string
	OutputDirectory string
}

func insertFilesIntoBoxRepository(boxRepository string) error {
	// Load configuration file
	configuration, err := loadConfigurationFromCLIOrCurrentDirectory()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Save configuration file
	if err := saveConfigurationToDirectory(configuration, boxRepository, "uvbox.toml"); err != nil {
		return fmt.Errorf("failed to copy configuration file to box repository: %w", err)
	}

	// Create wheels folder
	wheelsFolderPath := filepath.Join(boxRepository, "wheels")
	if err := os.MkdirAll(wheelsFolderPath, 0755); err != nil {
		return fmt.Errorf("failed to create wheels folder inside box repository: %w", err)
	}

	// Insert wheels
	if len(WheelsToEmbed) > 0 {
		for _, wheel := range WheelsToEmbed {
			if _, err := copyFileToFolder(wheel, filepath.Base(wheel), wheelsFolderPath); err != nil {
				return fmt.Errorf("failed to copy wheel file to box repository: %w", err)
			}
		}
	}

	// Get certificates bundle parameter from configuration file
	userCertsBundlePath, err := configurationCertificatesBundlePath()
	if err != nil {
		return fmt.Errorf("could not get configured user certificates bundle: %w", err)
	}

	// Insert it if specified
	if userCertsBundlePath != "" {
		_, err = copyFileToFolder(userCertsBundlePath, "ca-bundle.crt", boxRepository)
		if err != nil {
			return fmt.Errorf("failed to copy user certificates bundle to box repository: %w", err)
		}
	}

	return nil
}

func determineBuildsTargets() ([]string, []string) {
	// Build platform list
	platforms := []string{}
	if !Darwin && !Linux && !Windows {
		platforms = append(platforms, PLATFORM_DARWIN, PLATFORM_LINUX, PLATFORM_WINDOWS)
	} else {
		if Darwin {
			platforms = append(platforms, PLATFORM_DARWIN)
		}
		if Linux {
			platforms = append(platforms, PLATFORM_LINUX)
		}
		if Windows {
			platforms = append(platforms, PLATFORM_WINDOWS)
		}
	}

	// Build architecture list
	archs := []string{}
	if !Amd && !Arm {
		archs = append(archs, ARCH_AMD64, ARCH_ARM64)
	} else {
		if Amd {
			archs = append(archs, ARCH_AMD64)
		}
		if Arm {
			archs = append(archs, ARCH_ARM64)
		}
	}

	return platforms, archs
}

func run() error {
	// Diplay banner
	if !NoBanner {
		renderer := pterm.DefaultBigText.WithLetters(
			putils.LettersFromStringWithStyle("UV", pterm.FgCyan.ToStyle()),
			putils.LettersFromStringWithStyle("BOX", pterm.FgLightMagenta.ToStyle()))

		err := renderer.Render() // Render the big text to the terminal
		if err != nil {
			_ = fmt.Sprintln("failed to display banner: %w", err)
		}
	}

	// Create a temporary directory
	temporaryDirectory, err := os.MkdirTemp("", "uvbox-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}

	// Setup the repository that will be compiled
	boxRepository, err := extractBoxRepository(temporaryDirectory)
	if err != nil {
		return fmt.Errorf("failed to extract box repository: %w", err)
	} else {
		if err = insertFilesIntoBoxRepository(boxRepository); err != nil {
			return fmt.Errorf("failed to insert files required for the compilation into the box repository: %w", err)
		}
	}

	// Compile for every target
	platforms, archs := determineBuildsTargets()
	if err = buildExecutableforEveryTarget(platforms, archs, boxRepository); err != nil {
		return fmt.Errorf("failed to build for every target: %w", err)
	}

	// Display success message
	pterm.Success.Println(pterm.Green("Available at: ") + pterm.Magenta(Output))

	return nil
}

func buildExecutableforEveryTarget(platforms, archs []string, boxRepository string) error {
	executableName, err := configurationScriptName()
	if err != nil {
		return fmt.Errorf("could not get executable script name from configuration: %w", err)
	}

	for _, platform := range platforms {
		for _, arch := range archs {
			if err := buildWithSpinner(boxRepository, platform, arch, executableName); err != nil {
				return fmt.Errorf("failed to build for %s/%s: %w", platform, arch, err)
			}
		}
	}

	return nil
}

func determineBuildName(executableName, platform string) string {
	build_name := executableName
	if platform == PLATFORM_WINDOWS {
		build_name = fmt.Sprintf("%s.exe", executableName)
	}
	return build_name
}

func goGenerate(repository, platform, arch string) error {
	generateCmd := exec.Command("go", "generate")
	generateCmd.Dir = repository
	generateCmd.Env = append(
		os.Environ(),
		fmt.Sprintf("PYOS=%s", platform),
		fmt.Sprintf("PYARCH=%s", arch),
	)
	if err := generateCmd.Run(); err != nil {
		return fmt.Errorf("failed to run go generate command: %w", err)
	}

	return nil
}

func goBuild(repository, buildName, platform, arch string) error {
	var outbuf, errbuf strings.Builder

	// Compilation flag
	ldflags := "-ldflags=-s -w"
	if len(WheelsToEmbed) > 0 {
		ldflags += " -X main.INSTALL_WHEELS=yes"
	}

	// Command
	cmd := exec.Command("go", "build", "-o", buildName, ldflags)
	cmd.Dir = repository
	cmd.Env = append(
		os.Environ(),
		fmt.Sprintf("GOOS=%s", platform),
		fmt.Sprintf("GOARCH=%s", arch),
		"CGO_ENABLED=0",
	)
	cmd.Stdout = &outbuf
	cmd.Stderr = &errbuf

	// Run command
	if err := cmd.Run(); err != nil {
		stdout := outbuf.String()
		stderr := errbuf.String()
		return fmt.Errorf("failed to run go build command: %w\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}

	return nil
}

func checkBuiltExecutableExists(repository, buildName string) (string, error) {
	executable := filepath.Join(repository, buildName)
	if _, err := os.Stat(executable); os.IsNotExist(err) {
		// On Windows host, Go may add .exe suffix even when cross-compiling for other platforms.
		// Check for the .exe variant if the original file wasn't found.
		if runtime.GOOS == "windows" && !strings.HasSuffix(buildName, ".exe") {
			executableWithExe := executable + ".exe"
			if _, exeErr := os.Stat(executableWithExe); exeErr == nil {
				return executableWithExe, nil
			}
		}
		return "", fmt.Errorf("could not find built executable at %s: %w", executable, err)
	} else if err != nil {
		return "", fmt.Errorf("could not read build executable file '%s' metadata: %w", executable, err)
	}

	return executable, nil
}

func outputFolderAbsolutePath() (string, error) {
	// Get absolute path of output
	output := filepath.Clean(Output)
	output, err := filepath.Abs(output)
	if err != nil {
		return "", fmt.Errorf("could not get absolute path of output folder: %w", err)
	}

	return output, nil
}

func determineArchiveNameFromPlatform(executableName, platform, arch string) (string, error) {
	archiveName := ""
	switch platform {
	case PLATFORM_WINDOWS:
		switch arch {
		case ARCH_AMD64:
			archiveName = "x86_64-pc-windows-msvc.zip"
		case ARCH_ARM64:
			archiveName = "aarch64-pc-windows-msvc.zip"
		default:
			return "", fmt.Errorf("unsupported architecture: %s/%s", platform, arch)
		}
	case PLATFORM_LINUX:
		switch arch {
		case ARCH_AMD64:
			archiveName = "x86_64-unknown-linux-gnu.tar.gz"
		case ARCH_ARM64:
			archiveName = "aarch64-unknown-linux-gnu.tar.gz"
		default:
			return "", fmt.Errorf("unsupported architecture: %s/%s", platform, arch)
		}
	case PLATFORM_DARWIN:
		switch arch {
		case ARCH_AMD64:
			archiveName = "x86_64-apple-darwin.tar.gz"
		case ARCH_ARM64:
			archiveName = "aarch64-apple-darwin.tar.gz"
		default:
			return "", fmt.Errorf("unsupported architecture: %s/%s", platform, arch)
		}
	}

	archiveName = executableName + "-" + archiveName
	return archiveName, nil
}

func determineExecutableArchiveDestination(executableName, platform, arch string) (string, error) {
	output, err := outputFolderAbsolutePath()
	if err != nil {
		return "", fmt.Errorf("could not get executable archive output folder path: %w", err)
	}

	archiveName, err := determineArchiveNameFromPlatform(executableName, platform, arch)
	if err != nil {
		return "", fmt.Errorf("failed to determine executable archive name: %w", err)
	}

	// Archive the executable to the destination with the correct method (determine with filename)
	archiveDestination := filepath.Join(output, archiveName)
	return archiveDestination, nil
}

func deleteFileIfExists(filePath string) error {
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not remove existing file %s: %w", filePath, err)
	}
	return nil
}

func createFileArchiveWithFormatFromName(executable, archiveDestination, entryName string) error {
	if strings.HasSuffix(archiveDestination, ".zip") {
		if err := zipFile(executable, archiveDestination, entryName); err != nil {
			return fmt.Errorf("failed to create executable zip archive: %w", err)
		}
	} else if strings.HasSuffix(archiveDestination, ".tar.gz") {
		if err := gzipFile(executable, archiveDestination, entryName); err != nil {
			return fmt.Errorf("failed to create executable gzip archive: %w", err)
		}
	} else {
		return fmt.Errorf("could not determine archive file extension from: %s", archiveDestination)
	}

	return nil
}

func compileExecutable(repository, buildName, platform, arch string) (string, error) {
	// go generate command
	if err := goGenerate(repository, platform, arch); err != nil {
		return "", fmt.Errorf("failed to run go generate: %w", err)
	}

	// go build command
	if err := goBuild(repository, buildName, platform, arch); err != nil {
		return "", fmt.Errorf("failed to run go build: %w", err)
	}

	// Ensure build executable exists and is valid
	executable, err := checkBuiltExecutableExists(repository, buildName)
	if err != nil {
		return "", fmt.Errorf("failed to find existing built executable after go build: %w", err)
	}

	return executable, nil
}

func archiveExecutable(builtExecutablePath, executableName, platform, arch string) (string, error) {
	archiveDestination, err := determineExecutableArchiveDestination(executableName, platform, arch)
	if err != nil {
		return "", fmt.Errorf("failed to determine built executable archive destination path to create: %w", err)
	}

	// Remove archive if it already exists
	if err := deleteFileIfExists(archiveDestination); err != nil {
		return "", fmt.Errorf("failed to try to delete if exists %s: %w", archiveDestination, err)
	}

	// Determine the correct entry name for the archive (e.g., "app" for Linux, "app.exe" for Windows)
	// This ensures the file inside the archive has the correct name for the target platform,
	// even if the actual file on disk has a different name (e.g., when cross-compiling from Windows)
	entryName := determineBuildName(executableName, platform)

	// Use the correct method to archive the executable based on the archive extension
	if err := createFileArchiveWithFormatFromName(builtExecutablePath, archiveDestination, entryName); err != nil {
		return "", fmt.Errorf("failed to create executable archive: %w", err)
	}

	return archiveDestination, nil
}

func setupNfpmConfigurationFiles(temporaryDirectory, executableName string) (string, error) {
	// Copy the nfpm configuration file
	file, err := copyFileToFolder(Nfpm, "nfpm.yaml", temporaryDirectory)
	if err != nil {
		return "", fmt.Errorf("failed to copy nfpm configuration file to temporary directory: %w", err)
	}

	// Insert pre-remove script
	preRemoveScript := filepath.Join(temporaryDirectory, "pre_remove.sh")
	preRemoveScriptContent := fmt.Sprintf("#!/bin/sh\n%s self remove\n", executableName)
	if err := os.WriteFile(preRemoveScript, []byte(preRemoveScriptContent), 0755); err != nil {
		return "", fmt.Errorf("failed to write pre-remove script: %w", err)
	}

	return file, nil
}

func nfpmPkg(nfpmConfig, output, packager, temporaryDirectory string, environ []string) error {
	var outbuf, errbuf strings.Builder
	nfpmCmd := exec.Command("nfpm", "pkg", "--config", nfpmConfig, "--target", output, "--packager", packager)
	nfpmCmd.Dir = temporaryDirectory
	nfpmCmd.Stdout = &outbuf
	nfpmCmd.Stderr = &errbuf
	nfpmCmd.Env = environ
	if err := nfpmCmd.Run(); err != nil {
		return fmt.Errorf("failed to run nfpm command: %w\nstdout: %s\nstderr: %s", err, outbuf.String(), errbuf.String())
	}

	return nil
}

func nfpmEnvironmentVariables(builtExecutablePath, executableName, platform, arch, version string) []string {
	return append(
		os.Environ(),
		fmt.Sprintf("UVBOX_BUILT_EXECUTABLE=%s", builtExecutablePath),
		fmt.Sprintf("UVBOX_NAME=%s", executableName),
		fmt.Sprintf("UVBOX_PLATFORM=%s", platform),
		fmt.Sprintf("UVBOX_ARCH=%s", arch),
		fmt.Sprintf("UVBOX_VERSION=%s", version),
	)
}

func packageWithNfpm(packager, builtExecutablePath, executableName, platform, arch string) error {
	// Create a temporary working directory
	temporaryDirectory, err := os.MkdirTemp("", "uvbox-nfpm-")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory for nfpm: %w", err)
	}

	// Generate nfpm configuration file properly set
	nfpmConfig, err := setupNfpmConfigurationFiles(temporaryDirectory, executableName)
	if err != nil {
		return fmt.Errorf("failed to setup nfpm configuration file: %w", err)
	}

	// Get the output directory
	output, err := outputFolderAbsolutePath()
	if err != nil {
		return fmt.Errorf("failed to get output directory for nfpm: %w", err)
	}

	nfpmEnviron := nfpmEnvironmentVariables(builtExecutablePath, executableName, platform, arch, ReleaseVersion)

	// Run nfpm
	if err := nfpmPkg(nfpmConfig, output, packager, temporaryDirectory, nfpmEnviron); err != nil {
		return fmt.Errorf("failed to run nfpm package command: %w", err)
	}

	return nil
}

func packageExecutableAsDeb(builtExecutablePath, executableName, platform, arch string) error {
	if err := packageWithNfpm("deb", builtExecutablePath, executableName, platform, arch); err != nil {
		return fmt.Errorf("failed to package executable as deb: %w", err)
	}

	return nil
}

func packageExecutableAsRpm(builtExecutablePath, executableName, platform, arch string) error {
	if err := packageWithNfpm("rpm", builtExecutablePath, executableName, platform, arch); err != nil {
		return fmt.Errorf("failed to package executable as rpm: %w", err)
	}

	return nil
}

func packageExecutable(builtExecutablePath, executableName, platform, arch string) error {
	if err := packageExecutableAsDeb(builtExecutablePath, executableName, platform, arch); err != nil {
		return fmt.Errorf("failed to package executable for debian: %w", err)
	}

	if err := packageExecutableAsRpm(builtExecutablePath, executableName, platform, arch); err != nil {
		return fmt.Errorf("failed to package executable for rpm: %w", err)
	}
	return nil
}

func compileAndArchiveAndPackageExecutable(repository, executableName, platform, arch string) (string, bool, error) {
	// Add .exe suffix to built executable name if windows
	buildName := determineBuildName(executableName, platform)

	// Let's compile the executable
	executable, err := compileExecutable(repository, buildName, platform, arch)
	if err != nil {
		return "", false, fmt.Errorf("failed to compile executable: %w", err)
	}

	// And then archive the built executable
	archive, err := archiveExecutable(executable, executableName, platform, arch)
	if err != nil {
		return "", false, fmt.Errorf("failed to archive built executable: %w", err)
	}

	// Package the executable
	hasPackaged := false
	if Nfpm != "" && platform == PLATFORM_LINUX {
		if err = packageExecutable(executable, executableName, platform, arch); err != nil {
			return "", false, fmt.Errorf("failed to package executable: %w", err)
		} else {
			hasPackaged = true
		}
	}

	return archive, hasPackaged, nil
}

func buildWithSpinner(repository, platform, arch, executableName string) error {
	spinnerMessage := " 📦 Building for " + platform + "/" + arch
	spinner, _ := pterm.DefaultSpinner.Start(spinnerMessage)

	// Compile the executable
	archive, hasPackaged, err := compileAndArchiveAndPackageExecutable(repository, executableName, platform, arch)
	if err != nil {
		spinner.Fail(err)
		return fmt.Errorf("failed to compile & archive executable: %w", err)
	}

	// Success messages
	spinner.Success(fmt.Sprintf("%s/%s 👉 %s", strings.ToUpper(platform), strings.ToUpper(arch), archive)) // Resolve spinner with success message.
	if hasPackaged {
		successMessage := fmt.Sprintf("%s/%s 👉 Package(s) generated!", strings.ToUpper(platform), strings.ToUpper(arch))
		spinner, _ = pterm.DefaultSpinner.Start("")
		spinner.Success(successMessage)
	}

	return nil
}

func copyFileToFolder(source, name, destination string) (string, error) {
	// Read source file
	sourceFile, err := os.ReadFile(source)
	if err != nil {
		return "", fmt.Errorf("failed to read source file for copy %s: %w", source, err)
	}

	// Write to destination
	destinationFile := filepath.Join(destination, name)
	if err := os.WriteFile(destinationFile, sourceFile, 0755); err != nil {
		return "", fmt.Errorf("failed to write file for copy %s: %w", destinationFile, err)
	}

	return destinationFile, nil
}

func extractBoxRepository(destination string) (string, error) {
	err := extractGzipFolder(embeddedBoxRepository, destination)
	if err != nil {
		return "", fmt.Errorf("failed to extract box repository: %w", err)
	}

	boxRepositoryPath := filepath.Join(destination, "box")
	// Ensure box repository folder exists
	if _, err := os.Stat(boxRepositoryPath); os.IsNotExist(err) {
		return "", fmt.Errorf("box repository path does not exist: %w", err)
	} else if err != nil {
		return "", fmt.Errorf("failed to get box repository path: %w", err)
	}

	return boxRepositoryPath, nil
}

func gzipFile(source, destination, entryName string) error {
	parentFolder := filepath.Dir(destination)
	if err := ensureDirectory(parentFolder); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", parentFolder, err)
	}

	files, err := archives.FilesFromDisk(context.Background(), nil, map[string]string{
		source: entryName,
	})
	if err != nil {
		return fmt.Errorf("failed to prepare file %s for archiving: %w", source, err)
	}

	out, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("could not touch destination archive %s: %w", destination, err)
	}
	defer out.Close()

	format := archives.CompressedArchive{
		Compression: archives.Gz{},
		Archival:    archives.Tar{},
	}

	err = format.Archive(context.Background(), out, files)
	if err != nil {
		return fmt.Errorf("failed to archive %s into %s: %w", source, destination, err)
	}

	return nil
}

func zipFile(source, destination, entryName string) error {
	parentFolder := filepath.Dir(destination)
	if err := ensureDirectory(parentFolder); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", parentFolder, err)
	}

	// Create target file
	zipfile, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create target file: %w", err)
	}
	defer zipfile.Close()

	// Create zip writer
	archive := zip.NewWriter(zipfile)
	defer archive.Close()

	// Open source file
	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer file.Close()

	// Get file information
	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file information: %w", err)
	}

	// Create file in archive
	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("failed to create file info header: %w", err)
	}
	header.Name = entryName
	header.Method = zip.Deflate

	writer, err := archive.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("failed to create file in archive: %w", err)
	}

	// Copy contents
	if _, err = io.Copy(writer, file); err != nil {
		return fmt.Errorf("failed to copy contents: %w", err)
	}

	return nil
}

// extractGzipFolder extracts a tar.gz archive to a destination folder.
// It uses mholt/archives which auto-detects the archive format from the content.
func extractGzipFolder(gzipFile []byte, destination string) error {
	ctx := context.Background()
	reader := bytes.NewReader(gzipFile)

	// Auto-detect archive format (tar.gz, etc.) by peeking at the stream header
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

func ensureDirectory(destination string) error {
	// Check destination is not an existing file
	destinationInfo, err := os.Stat(destination)
	if err == nil && !destinationInfo.IsDir() {
		return fmt.Errorf("destination %s is not a directory", destination)
	}

	if err := os.MkdirAll(destination, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destination, err)
	}
	return nil
}
