# Constants for project structure
from dataclasses import dataclass
from pathlib import Path
import sys
from typing import List


PROJECT_ROOT = Path(
    __file__
).parents[2]  # Go up from pypi/uvbox/ to project root
BUILD_DIR = PROJECT_ROOT / "build"
DIST_DIR = PROJECT_ROOT / "wheels"


@dataclass(frozen=True)
class Platform:
    """
    Platform configuration for cross-platform wheel building.

    Attributes:
        binary_suffix: File extension for the binary on this platform (e.g., '.exe' for Windows)
        wheel_platform: Platform tag used in wheel filename according to PEP 427
        go_os: GOOS value for Go cross-compilation
        go_arch: GOARCH value for Go cross-compilation
    """

    binary_suffix: str
    wheel_platform: str
    go_os: str
    go_arch: str

    def get_binary_filename(self, binary_name: str) -> str:
        """
        Generate the full binary filename for this platform.

        Args:
            binary_name: Base name of the binary without extension

        Returns:
            Full binary filename including platform-specific extension
        """
        return f"{binary_name}{self.binary_suffix}"

    def __str__(self) -> str:
        """Return a string representation of the platform."""
        return f"{self.go_os}_{self.go_arch}"


# Platform mappings for wheel tags
# Reference: https://packaging.python.org/en/latest/specifications/platform-compatibility-tags/
SUPPORTED_PLATFORMS: List[Platform] = [
    Platform(
        binary_suffix="",
        wheel_platform="manylinux2014_x86_64",
        go_os="linux",
        go_arch="amd64",
    ),
    Platform(
        binary_suffix="",
        wheel_platform="manylinux2014_aarch64",
        go_os="linux",
        go_arch="arm64",
    ),
    Platform(
        binary_suffix="",
        wheel_platform="macosx_10_9_x86_64",
        go_os="darwin",
        go_arch="amd64",
    ),
    Platform(
        binary_suffix="",
        wheel_platform="macosx_11_0_arm64",
        go_os="darwin",
        go_arch="arm64",
    ),
    Platform(
        binary_suffix=".exe",
        wheel_platform="win_amd64",
        go_os="windows",
        go_arch="amd64",
    ),
    Platform(
        binary_suffix=".exe",
        wheel_platform="win_arm64",
        go_os="windows",
        go_arch="arm64",
    ),
]

def find_platform_by_go_target(go_os: str, go_arch: str) -> Platform:
    """
    Find a platform configuration matching the given Go OS and architecture.

    Args:
        go_os: Go operating system identifier (e.g., 'linux', 'darwin', 'windows')
        go_arch: Go architecture identifier (e.g., 'amd64', 'arm64')

    Returns:
        Matching Platform instance, or None if no match found
    """
    for platform in SUPPORTED_PLATFORMS:
        if platform.go_os == go_os and platform.go_arch == go_arch:
            return platform

    print(f"ERROR: Unsupported platform: {go_os}/{go_arch}")
    print(f"Supported platforms: {[str(p) for p in SUPPORTED_PLATFORMS]}")
    sys.exit(1)
