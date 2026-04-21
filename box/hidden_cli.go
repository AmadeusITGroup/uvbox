package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func runHiddenCliAndExitIfCalled(b *Box, packageVersion, packageConstraintsUrl string) {

	binaryName := "binary"
	if len(os.Args) > 0 {
		binaryName = os.Args[0]
	}

	var rootCmd = &cobra.Command{
		Use:   binaryName,
		Short: "Self management hidden commands",
		Args:  cobra.NoArgs,
	}

	var selfCmd = &cobra.Command{
		Use:   "self",
		Short: "Self management hidden commands",
		Args:  cobra.NoArgs,
	}

	var cacheCmd = &cobra.Command{
		Use:   "cache",
		Short: "Manage cache",
		Args:  cobra.NoArgs,
	}

	var cleanCacheCmd = &cobra.Command{
		Use:   "clean",
		Short: "Clean cache (uv)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			err := b.CleanCache()
			if err != nil {
				logger.Fatal("Failed to clean cache", logger.Args("error", err))
			}

			fmt.Println("Cache cleaned. 🧹")
		},
	}

	var pathCmd = &cobra.Command{
		Use:   "path",
		Short: "Display paths related to the installation",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("data folder: %s\n", b.uvboxHomePath())
			fmt.Printf("configuration folder: %s\n", b.dedicatedConfigurationFolderPath())
			fmt.Printf("uv folder: %s\n", b.dedicatedUvFolderPath())
			fmt.Printf("uv tools directory: %s\n", b.uvToolDirPath())
			fmt.Printf("uv tools binaries directory: %s\n", b.uvToolBinDirPath())
		},
	}

	var installCmd = &cobra.Command{
		Use:   "install",
		Short: "Ensure the package is installed with the latest version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if yes, err := b.NeedsInstall(); err != nil {
				logger.Fatal("Failed to check if box needs install", logger.Args("error", err))
			} else if yes {
				logger.Trace("Installing box")
				if err := b.InstallWithSpinner(); err != nil {
					logger.Fatal("Failed to install box", logger.Args("error", err))
				}
			}
		},
	}

	var updateCmd = &cobra.Command{
		Use:   "update",
		Short: "Update the package to the latest available version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			preUpdateVersion, err := b.InstalledPackageVersion()
			if err != nil {
				logger.Fatal("Failed to get installed version", logger.Args("package", b.PackageName, "error", err))
			}

			err = b.InstallPackageWithSpinner(packageVersion, packageConstraintsUrl)
			if err != nil {
				logger.Fatal("Failed to install", logger.Args("package", b.PackageName, "error", err))
			}

			postUpdateVersion, err := b.InstalledPackageVersion()
			if err != nil {
				logger.Fatal("Failed to get installed version", logger.Args("package", b.PackageName, "error", err))
			}

			if preUpdateVersion == postUpdateVersion {
				fmt.Println("Already up-to-date. 🎉")
			} else {
				fmt.Printf("Updated from %s to %s. 🚀\n", preUpdateVersion, postUpdateVersion)
			}
		},
	}

	var uvCmd = &cobra.Command{
		Use:   "uv",
		Short: "Run uv executable with the isolated environment",
		Args:  cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			_, err := b.RunUv(args)
			if err != nil {
				logger.Fatal("Failed to run uv", logger.Args("error", err))
			}
		},
	}

	var removeCmd = &cobra.Command{
		Use:   "remove",
		Short: "Remove installation",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			err := b.Uninstall()
			if err != nil {
				logger.Fatal("Failed to uninstall", logger.Args("error", err))
			}

			fmt.Println("Removed. 👋")
		},
	}

	cacheCmd.AddCommand(cleanCacheCmd)
	selfCmd.AddCommand(installCmd)
	selfCmd.AddCommand(updateCmd)
	selfCmd.AddCommand(removeCmd)
	selfCmd.AddCommand(cacheCmd)
	selfCmd.AddCommand(pathCmd)
	selfCmd.AddCommand(uvCmd)
	rootCmd.AddCommand(selfCmd)

	if len(os.Args) > 1 {
		if os.Args[1] == "self" {
			logger.Trace("Running hidden CLI command")

			err := rootCmd.Execute()
			if err != nil {
				logger.Fatal("Failed to run CLI", logger.Args("error", err))
			}

			// Exit the whole program after hidden CLI command
			os.Exit(0)
		}
	} else {
		logger.Trace("No hidden CLI command called")
	}
}
