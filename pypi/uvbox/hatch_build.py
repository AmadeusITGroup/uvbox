"""
Hatch build hooks for creating Python wheels containing Go binaries.

This module provides custom build and metadata hooks for hatchling that:
1. Include Go binaries as console scripts in Python wheels
2. Handle dynamic metadata like version, license, and README
3. Support cross-platform binary packaging

The hooks read configuration from environment variables set by the build script.
"""

import os
import sys
from pathlib import Path
from typing import Any, Dict, override

from hatchling.builders.config import BuilderConfig
from hatchling.builders.hooks.plugin.interface import BuildHookInterface
from hatchling.metadata.plugin.interface import MetadataHookInterface


def validate_required_file(file_path: Path) -> None:
    """
    Validate that a required file exists and provide helpful error information.

    Args:
        file_path: Path to the file that should exist

    Raises:
        SystemExit: If the file doesn't exist
    """
    if not file_path.exists():
        print(f"ERROR: Required file not found: {file_path.name}")
        if file_path.parent.exists():
            available_files = [f.name for f in file_path.parent.glob("*")]
            print(f"Available files in {file_path.parent}: {' '.join(available_files)}")
        sys.exit(1)


def get_binary_file_path() -> Path:
    """
    Get the path to the binary file from environment variables.

    Returns:
        Path to the binary file that should be included in the wheel

    Raises:
        SystemExit: If the environment variable is not set or file doesn't exist
    """
    binary_path_str = os.environ.get("UVBOX_BINARY_FILE")
    if not binary_path_str:
        print("ERROR: UVBOX_BINARY_FILE environment variable not set")
        print(
            "This variable should contain the path to the binary to include in the wheel"
        )
        sys.exit(1)

    binary_path = Path(binary_path_str)
    validate_required_file(binary_path)
    return binary_path


def get_binary_name() -> str:
    """
    Get the binary name from environment variables.

    Returns:
        Name to use for the console script

    Raises:
        SystemExit: If the environment variable is not set
    """
    binary_name = os.environ.get("UVBOX_BINARY_NAME")
    if not binary_name:
        print("ERROR: UVBOX_BINARY_NAME environment variable not set")
        print("This variable should contain the name for the console script")
        sys.exit(1)
    return binary_name


class CustomBuildHook(BuildHookInterface[BuilderConfig]):
    """
    Custom build hook to include Go binaries in Python wheels.

    This hook reads the binary path and name from environment variables
    and configures the wheel to include the binary as a console script.
    """

    @override
    def initialize(self, version: str, build_data: Dict[str, Any]) -> None:
        """
        Initialize the build hook and configure binary inclusion.

        Args:
            version: Package version being built
            build_data: Build configuration data to modify
        """
        binary_path = get_binary_file_path()
        binary_name = get_binary_name()

        # Configure build metadata
        build_data["tag"] = os.environ.get("UVBOX_TAG", version)
        build_data["pure_python"] = False  # Contains native binaries

        # Determine console script name based on platform
        console_script_name = binary_name
        if binary_path.suffix == ".exe" and not binary_name.endswith(".exe"):
            console_script_name = f"{binary_name}.exe"

        # Map the binary to a console script
        build_data["shared_scripts"] = {str(binary_path): console_script_name}

        # Include SBOM file if provided
        sbom_path_str = os.environ.get("UVBOX_SBOM_FILE")
        if not sbom_path_str:
            print("ERROR: UVBOX_SBOM_FILE environment variable not set")
            print(
                "This variable should contain the path to the sbom to include in the wheel"
            )
            sys.exit(1)
        sbom_path = Path(sbom_path_str)
        validate_required_file(sbom_path)
        build_data["sbom_files"].append(str(sbom_path))


class CustomMetadataHook(MetadataHookInterface):
    """Custom metadata hook to set dynamic metadata"""

    @override
    def update(self, metadata: dict[str, Any]) -> None:
        # Find project root (go up from pypi/uvbox/ to project root)
        project = Path(__file__).resolve().parent.parent.parent

        # # Get license file from project root
        # license_file = project / "LICENSE"
        # validate_required_file(license_file)

        # # Get README file from project root
        # readme_file = project / "README.md"
        # validate_required_file(readme_file)

        # # Set metadata with absolute paths
        # metadata["license-files"] = [str(license_file.resolve())]
        # metadata["readme"] = str(readme_file.resolve())

        # Handle version
        version = os.environ.get("UVBOX_VERSION", "0.1.0")
        metadata["version"] = version
