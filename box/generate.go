//go:build generate

package main

import (
	"log"
	"os"
	"path/filepath"
)

var CONFIGURATION_FILENAME = "uvbox.toml"
var CERTIFICATES_BUNDLE_FILENAME = "ca-bundle.crt"
var WHEELS_FOLDER = "wheels"
var WHEELS_PLACEHOLDER = filepath.Join(WHEELS_FOLDER, "placeholder")

func deleteIfExists(filename string) {
	if _, err := os.Stat(filename); err != nil && os.IsNotExist(err) {
		return
	} else if err != nil {
		log.Fatalf("failed to check if %s exists: %w", filename, err)
	}
}

func generateEmptyFileIfMissing(filename string) {
	if _, err := os.Stat(filename); err != nil && os.IsNotExist(err) {
		_, err := os.Create(filename)
		if err != nil {
			log.Fatalf("failed to create file %s: %s", filename, err)
		}
	} else if err != nil {
		log.Fatalf("failed to check if %s exists: %s", filename, err)
	}
}

func generateEmptyFolderIfMissing(folder string) {
	if err := os.MkdirAll(folder, 0755); err != nil && !os.IsExist(err) {
		log.Fatalf("failed to create folder %s: %s", folder, err)
	}
}

func main() {
	// Delete and generate empty configuration file
	generateEmptyFileIfMissing(CONFIGURATION_FILENAME)

	// Delete and generate empty certificates bundle file
	generateEmptyFileIfMissing(CERTIFICATES_BUNDLE_FILENAME)

	// Generate empty wheels folder
	generateEmptyFolderIfMissing(WHEELS_FOLDER)

	// Generate wheels folder placeholder
	generateEmptyFileIfMissing(WHEELS_PLACEHOLDER)
}
