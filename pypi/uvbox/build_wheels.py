#!/usr/bin/env python3
"""
Build script for creating Python wheels containing Go binaries.

This script builds Python wheels that include pre-built Go binaries as console scripts.
It is designed for integration with GoReleaser, which provides the pre-built binaries.

The script is generic and can be used for any Go binary project by specifying the
binary name as a command-line argument.

Inspired by python-nfpm: https://gitlab.com/vmeurisse/python-nfpm
"""

import os
import shutil
import subprocess
import sys
from argparse import ArgumentParser
from pathlib import Path
from typing import Optional

from sbom import generate_sbom
from utils import (
    BUILD_DIR,
    DIST_DIR,
    PROJECT_ROOT,
    Platform,
    find_platform_by_go_target,
)


def build_python_wheel(
    binary_path: Path, platform: Platform, version: str, binary_name: str
) -> None:
    """
    Build a Python wheel containing the specified binary for the given platform.

    Args:
        binary_path: Path to the compiled binary file
        platform: Target platform configuration
        version: Version string for the wheel (must be PEP 440 compliant)
        binary_name: Name of the binary (used for console script name)

    Raises:
        subprocess.CalledProcessError: If the wheel build process fails
    """
    # Create platform-specific build directory
    wheel_build_dir = BUILD_DIR / f"wheel_{platform.go_os}_{platform.go_arch}"
    if wheel_build_dir.exists():
        shutil.rmtree(wheel_build_dir)
    wheel_build_dir.mkdir(parents=True)

    # Copy necessary build files to the build directory
    pypi_source_dir = PROJECT_ROOT / "pypi" / "uvbox"
    build_files = ["pyproject.toml", "hatch_build.py"]
    metadata_files = ["LICENSE", "README.md"]

    # Copy pyproject.toml and build script to build directory
    for build_file in build_files:
        destination_path = wheel_build_dir / build_file
        if destination_path.exists():
            destination_path.unlink()
        destination_path.symlink_to(pypi_source_dir / build_file)

    # Copy README.md and LICENSE to build directory
    for metadata_file in metadata_files:
        destination_path = wheel_build_dir / metadata_file
        if destination_path.exists():
            destination_path.unlink()
        destination_path.symlink_to(PROJECT_ROOT / metadata_file)

    # Set environment variables for the hatch build process
    build_env = os.environ.copy()
    # Generate SBOM for the target platform
    sbom_path = wheel_build_dir / f"{binary_name}.cdx.json"
    generate_sbom(platform, sbom_path)

    build_env.update(
        {
            "UVBOX_BINARY_FILE": str(binary_path),
            "UVBOX_TAG": f"py3-none-{platform.wheel_platform}",
            "UVBOX_VERSION": version,
            "UVBOX_BINARY_NAME": binary_name,
            "UVBOX_SBOM_FILE": str(sbom_path),
        }
    )

    # Build the wheel using hatchling
    subprocess.run(
        ["hatchling", "build", "--target", "wheel", "--directory", str(DIST_DIR)],
        check=True,
        env=build_env,
        cwd=wheel_build_dir,
    )


def normalize_version_for_pep440(version: str) -> str:
    """
    Normalize a version string to be PEP 440 compliant.

    Args:
        version: Raw version string (may contain SNAPSHOT, leading 'v', etc.)

    Returns:
        PEP 440 compliant version string
    """
    # Strip leading 'v' from tag
    normalized_version = version.lstrip("v")

    # Convert snapshot versions to development releases
    if "SNAPSHOT" in normalized_version:
        # Convert "0.0.0-SNAPSHOT-abc123" to "0.0.0.dev0+abc123"
        normalized_version = normalized_version.replace("-SNAPSHOT-", ".dev0+")

    return normalized_version


def build_single_platform_wheel(
    go_os: str,
    go_arch: str,
    version: str,
    binary_artifact_path: str,
    binary_name: Optional[str] = None,
) -> None:
    """
    Build a wheel for a single platform using an existing binary artifact.

    This function is primarily used for GoReleaser integration where binaries
    are already built and we need to package them into wheels.

    Args:
        go_os: Go operating system identifier
        go_arch: Go architecture identifier
        version: Version string for the wheel
        binary_artifact_path: Path to the pre-built binary artifact
        binary_name: Name for the binary (if None, derived from artifact filename)

    Raises:
        SystemExit: If the platform is unsupported or binary file is missing
    """
    # Find the matching platform configuration
    platform = find_platform_by_go_target(go_os, go_arch)

    # Validate that the binary artifact exists
    binary_path = Path(binary_artifact_path)
    if not binary_path.exists():
        print(f"ERROR: Binary artifact not found: {binary_artifact_path}")
        sys.exit(1)

    # Derive binary name from artifact path if not provided
    if binary_name is None:
        binary_name = binary_path.stem  # Remove file extension

    # Normalize version for PEP 440 compliance
    normalized_version = normalize_version_for_pep440(version)

    # Build the wheel
    build_python_wheel(binary_path, platform, normalized_version, binary_name)


def main() -> None:
    """
    Main entry point for the wheel building script.

    Builds a wheel for a single platform using a pre-built binary artifact.
    """
    parser = ArgumentParser(
        description="Build Python wheel for a pre-built Go binary (GoReleaser integration)",
        epilog="This script creates a Python wheel containing a pre-built Go binary as a console script.",
    )

    parser.add_argument("os", help="Target operating system (linux, darwin, windows)")
    parser.add_argument("arch", help="Target architecture (amd64, arm64)")
    parser.add_argument("version", help="Version string for the wheel")
    parser.add_argument("artifact_path", help="Path to the pre-built binary artifact")
    parser.add_argument(
        "--binary-name",
        help="Name for the binary console script (if not provided, derived from artifact filename)",
    )

    args = parser.parse_args()

    build_single_platform_wheel(
        args.os, args.arch, args.version, args.artifact_path, args.binary_name
    )


if __name__ == "__main__":
    main()
