from argparse import ArgumentParser
import os
from pathlib import Path
import shutil
import subprocess

from utils import BUILD_DIR, PROJECT_ROOT, Platform, find_platform_by_go_target


def _create_sbom_cache(platform: Platform) -> Path:
    output_path = BUILD_DIR / f"{platform.go_os}_{platform.go_arch}.cdx.json"
    if not output_path.exists():
        BUILD_DIR.mkdir(parents=True, exist_ok=True)

        sbom_env = os.environ.copy()
        sbom_env.update(
            {
                "GOOS": platform.go_os,
                "GOARCH": platform.go_arch,
            }
        )

        subprocess.run(
            [
                "cyclonedx-gomod",
                "app",
                "-json",
                "-output",
                output_path,
                PROJECT_ROOT / "boxer",
            ],
            check=True,
            env=sbom_env,
            cwd=PROJECT_ROOT,
        )
    return output_path


def generate_sbom(platform: Platform, output_path: Path):
    """
    Generate a CycloneDX SBOM for the Go module using cyclonedx-gomod.

    Args:
        platform: Target platform configuration (used to set GOOS/GOARCH build constraints)
        output_path: Path to write the SBOM JSON file

    Raises:
        subprocess.CalledProcessError: If the SBOM generation fails
    """
    sbom_file = _create_sbom_cache(platform)
    shutil.copy2(sbom_file, output_path)


def _main():
    parser = ArgumentParser(
        description="Generate a sbom for boxer project",
    )

    parser.add_argument("--os", required=True, help="Target operating system (linux, darwin, windows)")
    parser.add_argument("--arch", required=True, help="Target architecture (amd64, arm64)")
    parser.add_argument("--output", required=True, help="Path for the generated sbom")

    args = parser.parse_args()

    platform = find_platform_by_go_target(args.os, args.arch)
    generate_sbom(platform, Path(args.output))


if __name__ == "__main__":
    _main()
