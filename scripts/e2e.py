#!/usr/bin/env python3
"""
uvbox end-to-end test

Builds boxer from source, cross-compiles the cowsay example for all
platform/arch targets, extracts the native binary, and runs it end-to-end.

Two modes:
  python scripts/e2e.py pypi    # full cross-compile matrix, pypi install path
  python scripts/e2e.py wheel   # native target only, wheel install path
  python scripts/e2e.py         # both (pypi then wheel)

Requirements: Go toolchain (managed by mise). No Python dependencies
beyond the stdlib; the wheel flow fetches a cowsay wheel from PyPI via
urllib.
"""

import argparse
import json
import os
import platform
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
import urllib.request
import zipfile
from contextlib import contextmanager
from pathlib import Path

# Force UTF-8 output on Windows (default cp1252 can't encode box-drawing characters).
if sys.stdout.encoding != "utf-8":
    sys.stdout.reconfigure(encoding="utf-8")
if sys.stderr.encoding != "utf-8":
    sys.stderr.reconfigure(encoding="utf-8")

# ── Constants ────────────────────────────────────────────────────────────────

REPO_ROOT = Path(__file__).resolve().parent.parent
BOXER_DIR = REPO_ROOT / "boxer"
BOX_DIR = REPO_ROOT / "box"
EXAMPLE_CONFIG = REPO_ROOT / "examples" / "pypi" / "simple-app.toml"
EXECUTABLE_NAME = "cowsay"

TARGETS = [
    ("linux", "amd64", "x86_64-unknown-linux-gnu.tar.gz"),
    ("linux", "arm64", "aarch64-unknown-linux-gnu.tar.gz"),
    ("darwin", "amd64", "x86_64-apple-darwin.tar.gz"),
    ("darwin", "arm64", "aarch64-apple-darwin.tar.gz"),
    ("windows", "amd64", "x86_64-pc-windows-msvc.zip"),
    ("windows", "arm64", "aarch64-pc-windows-msvc.zip"),
]

PYPI_COWSAY_URL = "https://pypi.org/pypi/cowsay/json"


# ── Display ──────────────────────────────────────────────────────────────────


class Display:
    """Minimal terminal display with optional ANSI colors."""

    WIDTH = 56

    def __init__(self, total_steps: int):
        self.total_steps = total_steps
        # Enable ANSI escape codes on Windows Terminal.
        if sys.platform == "win32":
            os.system("")

    # ── ANSI helpers ──

    def _c(self, code: str, text: str) -> str:
        return f"\033[{code}m{text}\033[0m"

    def green(self, t: str) -> str:
        return self._c("32", t)

    def red(self, t: str) -> str:
        return self._c("31", t)

    def cyan(self, t: str) -> str:
        return self._c("36", t)

    def bold(self, t: str) -> str:
        return self._c("1", t)

    def dim(self, t: str) -> str:
        return self._c("90", t)

    # ── Structural output ──

    def header(self, title: str, subtitle: str = "") -> None:
        print()
        print(f"  {'─' * self.WIDTH}")
        print(f"  {self.bold(title)}")
        if subtitle:
            print(f"  {self.dim(subtitle)}")
        print(f"  {'─' * self.WIDTH}")

    def step(self, number: int, text: str) -> None:
        print(f"\n  {self.cyan(f'[{number}/{self.total_steps}]')} {self.bold(text)}")

    @contextmanager
    def task(self, label: str, width: int = 42):
        """Print a task label, yield a list for optional detail, then print OK/FAIL."""
        padded = label + " " + "." * max(1, width - len(label) - 1) + " "
        print(f"        {padded}", end="", flush=True)
        detail = []  # caller can append a string to override the default elapsed time
        t0 = time.monotonic()
        try:
            yield detail
            info = detail[0] if detail else f"({format_time(time.monotonic() - t0)})"
            print(self.green("OK") + (f"  {self.dim(info)}" if info else ""))
        except Exception as exc:
            msg = self.red("FAIL")
            if str(exc):
                msg += f"  {self.dim(str(exc))}"
            print(msg)
            if isinstance(exc, subprocess.CalledProcessError):
                if exc.stdout and exc.stdout.strip():
                    print(f"        stdout: {exc.stdout.strip()}")
                if exc.stderr and exc.stderr.strip():
                    print(f"        stderr: {exc.stderr.strip()}")
            raise

    def result(self, success: bool) -> None:
        print()
        print(f"  {'─' * self.WIDTH}")
        if success:
            print(f"  {self.green('All checks passed!')}")
        else:
            print(f"  {self.red('Some checks failed.')}")
        print(f"  {'─' * self.WIDTH}")
        print()


# ── Helpers ──────────────────────────────────────────────────────────────────


def detect_native_platform() -> tuple[str, str]:
    """Return (go_os, go_arch) for the current machine."""
    os_map = {"linux": "linux", "darwin": "darwin", "windows": "windows"}
    arch_map = {
        "x86_64": "amd64",
        "amd64": "amd64",
        "arm64": "arm64",
        "aarch64": "arm64",
    }

    os_name = platform.system().lower()
    machine = platform.machine().lower()

    go_os = os_map.get(os_name)
    go_arch = arch_map.get(machine)

    if not go_os:
        raise RuntimeError(f"Unsupported OS: {os_name}")
    if not go_arch:
        raise RuntimeError(f"Unsupported architecture: {machine}")

    return go_os, go_arch


def run(
    args: list[str], cwd: Path | None = None, timeout: int = 300
) -> subprocess.CompletedProcess:
    """Run a command, capture output, raise CalledProcessError on failure."""
    result = subprocess.run(
        args, cwd=cwd, capture_output=True, text=True, timeout=timeout
    )
    if result.returncode != 0:
        raise subprocess.CalledProcessError(
            result.returncode, args, output=result.stdout, stderr=result.stderr
        )
    return result


def format_size(size_bytes: int) -> str:
    return f"{size_bytes / (1024 * 1024):.1f} MB"


def format_time(seconds: float) -> str:
    return f"{seconds:.1f}s"


def _archive_members(archive: Path) -> list[str]:
    """Return the list of filenames inside an archive."""
    if archive.name.endswith(".tar.gz"):
        with tarfile.open(archive, "r:gz") as tf:
            return tf.getnames()
    elif archive.name.endswith(".zip"):
        with zipfile.ZipFile(archive) as zf:
            return zf.namelist()
    return []


def _extract_archive(archive: Path, destination: Path) -> None:
    """Extract a .tar.gz or .zip archive into the destination directory."""
    if archive.name.endswith(".tar.gz"):
        with tarfile.open(archive, "r:gz") as tf:
            tf.extractall(destination, filter="data")
    elif archive.name.endswith(".zip"):
        with zipfile.ZipFile(archive) as zf:
            zf.extractall(destination)
    else:
        raise RuntimeError(f"unknown archive format: {archive.name}")


# ── Shared steps ─────────────────────────────────────────────────────────────


def step_build_boxer(d: Display, step_number: int, work_dir: Path) -> Path:
    """Build the boxer (uvbox) binary from source.

    Order matters: box/ must be generated first because boxer's generate step
    bundles the box/ directory into an embedded archive.
    """
    d.step(step_number, "Building boxer")

    # Generate placeholder files in box/ (uvbox.toml, ca-bundle.crt, wheels/, etc.)
    with d.task("go generate (box)"):
        run(["go", "generate"], cwd=BOX_DIR)

    # Bundle box/ into embedded_box.tar.gz for boxer to embed at compile time
    with d.task("go generate (boxer)"):
        run(["go", "generate"], cwd=BOXER_DIR)

    # Compile the boxer binary
    boxer_name = "uvbox.exe" if sys.platform == "win32" else "uvbox"
    boxer_path = work_dir / boxer_name

    with d.task("go build"):
        run(["go", "build", "-o", str(boxer_path), "."], cwd=BOXER_DIR)

    return boxer_path


def step_extract_native(
    d: Display,
    step_number: int,
    archive_dir: Path,
    work_dir: Path,
    go_os: str,
    go_arch: str,
) -> Path:
    """Extract the archive matching the current host platform."""
    d.step(step_number, f"Extracting native binary ({go_os}/{go_arch})")

    suffix = next(s for o, a, s in TARGETS if o == go_os and a == go_arch)
    archive = archive_dir / f"{EXECUTABLE_NAME}-{suffix}"
    extract_dir = work_dir / "run"
    extract_dir.mkdir(exist_ok=True)

    with d.task(archive.name) as detail:
        _extract_archive(archive, extract_dir)
        detail.append("")

    exe_name = f"{EXECUTABLE_NAME}.exe" if go_os == "windows" else EXECUTABLE_NAME
    exe_path = extract_dir / exe_name

    if not exe_path.exists():
        raise FileNotFoundError(f"Executable not found after extraction: {exe_path}")

    if go_os != "windows":
        exe_path.chmod(0o755)

    return exe_path


def step_run_e2e(
    d: Display, step_number: int, exe_path: Path, test_phrase: str
) -> None:
    """Run the extracted binary end-to-end.

    This exercises the full runtime chain: the binary bootstraps uv, uses it
    to install cowsay into an isolated environment (from pypi or from an
    embedded wheel, depending on build mode), then invokes cowsay. We verify
    both a zero exit code and that the expected output appears.
    """
    d.step(step_number, "Running end-to-end")

    # Execute the binary — on a clean machine this triggers uv bootstrap + pip install
    with d.task(f'{exe_path.name} -t "{test_phrase}"'):
        result = subprocess.run(
            [str(exe_path), "-t", test_phrase],
            capture_output=True,
            text=True,
            timeout=120,
        )
        if result.returncode != 0:
            raise subprocess.CalledProcessError(
                result.returncode,
                [str(exe_path), "-t", test_phrase],
                output=result.stdout,
                stderr=result.stderr,
            )

    # Verify the cowsay output actually contains our test phrase
    with d.task("Output contains test phrase") as detail:
        combined = result.stdout + result.stderr
        if test_phrase not in combined:
            raise RuntimeError(f"'{test_phrase}' not found in output")
        detail.append("")


# ── pypi-flow steps ──────────────────────────────────────────────────────────


def step_cross_compile_pypi(
    d: Display, step_number: int, boxer_path: Path, work_dir: Path
) -> Path:
    """Use boxer to cross-compile the cowsay example for all 6 targets (pypi mode).

    A single invocation of `uvbox pypi` produces archives for every OS/arch
    combination (linux, darwin, windows × amd64, arm64). This validates that
    cross-compilation works from the current host.
    """
    d.step(step_number, "Cross-compiling cowsay for all targets")

    output_dir = work_dir / "archives"

    with d.task(f"uvbox pypi ({len(TARGETS)} targets)"):
        run(
            [
                str(boxer_path),
                "pypi",
                "--config",
                str(EXAMPLE_CONFIG),
                "-o",
                str(output_dir),
                "--no-banner",
            ]
        )

    return output_dir


def step_verify_archives_all(d: Display, step_number: int, archive_dir: Path) -> None:
    """Check that all 6 platform/arch archives exist and contain the expected executable."""
    d.step(step_number, "Verifying archives")

    for go_os, go_arch, suffix in TARGETS:
        archive = archive_dir / f"{EXECUTABLE_NAME}-{suffix}"
        exe_name = f"{EXECUTABLE_NAME}.exe" if go_os == "windows" else EXECUTABLE_NAME

        with d.task(f"{go_os}/{go_arch}") as detail:
            if not archive.exists():
                raise FileNotFoundError(f"not found: {archive.name}")

            members = _archive_members(archive)
            if exe_name not in members:
                raise RuntimeError(
                    f"executable '{exe_name}' not found in {archive.name}, "
                    f"contents: {members}"
                )

            detail.append(f"({format_size(archive.stat().st_size)})")


# ── wheel-flow steps ─────────────────────────────────────────────────────────


def _native_target_flags(go_os: str, go_arch: str) -> list[str]:
    """Return the boxer CLI flags to restrict compilation to the native target.

    boxer exposes --darwin/--linux/--windows for OS and --amd/--arm for arch.
    We pass only the pair matching the host to keep wheel-mode e2e fast; the
    pypi flow already exercises the full cross-compile matrix.
    """
    os_flag = {"linux": "--linux", "darwin": "--darwin", "windows": "--windows"}[go_os]
    arch_flag = {"amd64": "--amd", "arm64": "--arm"}[go_arch]
    return [os_flag, arch_flag]


def step_download_cowsay_wheel(d: Display, step_number: int, work_dir: Path) -> Path:
    """Download the latest py3-none-any cowsay wheel from PyPI.

    Using a real wheel (rather than building a local fixture) keeps the test
    subject identical to the pypi flow, so differences in behavior can be
    attributed to the install path rather than the package under test.
    """
    d.step(step_number, "Downloading cowsay wheel from PyPI")

    wheel_dir = work_dir / "wheels"
    wheel_dir.mkdir(exist_ok=True)

    with d.task("query pypi.org") as detail:
        with urllib.request.urlopen(PYPI_COWSAY_URL, timeout=30) as resp:
            metadata = json.loads(resp.read().decode("utf-8"))

        urls = metadata.get("urls") or []
        wheel_entry = next(
            (u for u in urls if u.get("filename", "").endswith("-py3-none-any.whl")),
            None,
        )
        if wheel_entry is None:
            raise RuntimeError("no py3-none-any wheel found for cowsay on PyPI")
        detail.append(f"({wheel_entry['filename']})")

    wheel_path = wheel_dir / wheel_entry["filename"]

    with d.task("download wheel") as detail:
        with urllib.request.urlopen(wheel_entry["url"], timeout=60) as resp:
            wheel_path.write_bytes(resp.read())
        detail.append(f"({format_size(wheel_path.stat().st_size)})")

    return wheel_path


def step_cross_compile_wheel(
    d: Display,
    step_number: int,
    boxer_path: Path,
    wheel_path: Path,
    work_dir: Path,
    go_os: str,
    go_arch: str,
) -> Path:
    """Build a single-target binary via `uvbox wheel` embedding the downloaded wheel.

    This exercises the ldflags `-X main.INSTALL_WHEELS=yes` path, which is the
    one that silently broke in commit f19680f (GOFLAGS parsing bug). Targeting
    only the native platform keeps e2e time reasonable; the pypi flow already
    exercises cross-compilation end-to-end.
    """
    d.step(step_number, f"Building cowsay wheel binary ({go_os}/{go_arch})")

    output_dir = work_dir / "archives"

    with d.task("uvbox wheel (native target)"):
        run(
            [
                str(boxer_path),
                "wheel",
                str(wheel_path),
                "--config",
                str(EXAMPLE_CONFIG),
                "-o",
                str(output_dir),
                "--no-banner",
                *_native_target_flags(go_os, go_arch),
            ]
        )

    return output_dir


def step_verify_archive_native(
    d: Display, step_number: int, archive_dir: Path, go_os: str, go_arch: str
) -> None:
    """Verify the single native-platform archive exists and is well-formed."""
    d.step(step_number, "Verifying archive")

    suffix = next(s for o, a, s in TARGETS if o == go_os and a == go_arch)
    archive = archive_dir / f"{EXECUTABLE_NAME}-{suffix}"
    exe_name = f"{EXECUTABLE_NAME}.exe" if go_os == "windows" else EXECUTABLE_NAME

    with d.task(f"{go_os}/{go_arch}") as detail:
        if not archive.exists():
            raise FileNotFoundError(f"not found: {archive.name}")

        members = _archive_members(archive)
        if exe_name not in members:
            raise RuntimeError(
                f"executable '{exe_name}' not found in {archive.name}, "
                f"contents: {members}"
            )

        detail.append(f"({format_size(archive.stat().st_size)})")


# ── Flows ────────────────────────────────────────────────────────────────────


def run_pypi(go_os: str, go_arch: str) -> bool:
    d = Display(total_steps=5)
    d.header("uvbox end-to-end test (pypi)", f"Platform: {go_os}/{go_arch}")

    work_dir = Path(tempfile.mkdtemp(prefix="uvbox-e2e-pypi-"))
    success = False

    try:
        boxer_path = step_build_boxer(d, 1, work_dir)
        archive_dir = step_cross_compile_pypi(d, 2, boxer_path, work_dir)
        step_verify_archives_all(d, 3, archive_dir)
        exe_path = step_extract_native(d, 4, archive_dir, work_dir, go_os, go_arch)
        step_run_e2e(d, 5, exe_path, "pypi-e2e-test")
        success = True
    except Exception as exc:
        if not isinstance(
            exc, (FileNotFoundError, RuntimeError, subprocess.CalledProcessError)
        ):
            print(f"\n        Error: {exc}")
    finally:
        shutil.rmtree(work_dir, ignore_errors=True)

    d.result(success)
    return success


def run_wheel(go_os: str, go_arch: str) -> bool:
    d = Display(total_steps=6)
    d.header("uvbox end-to-end test (wheel)", f"Platform: {go_os}/{go_arch}")

    work_dir = Path(tempfile.mkdtemp(prefix="uvbox-e2e-wheel-"))
    success = False

    try:
        boxer_path = step_build_boxer(d, 1, work_dir)
        wheel_path = step_download_cowsay_wheel(d, 2, work_dir)
        archive_dir = step_cross_compile_wheel(
            d, 3, boxer_path, wheel_path, work_dir, go_os, go_arch
        )
        step_verify_archive_native(d, 4, archive_dir, go_os, go_arch)
        exe_path = step_extract_native(d, 5, archive_dir, work_dir, go_os, go_arch)
        step_run_e2e(d, 6, exe_path, "wheel-e2e-test")
        success = True
    except Exception as exc:
        if not isinstance(
            exc, (FileNotFoundError, RuntimeError, subprocess.CalledProcessError)
        ):
            print(f"\n        Error: {exc}")
    finally:
        shutil.rmtree(work_dir, ignore_errors=True)

    d.result(success)
    return success


# ── Main ─────────────────────────────────────────────────────────────────────


def main() -> int:
    parser = argparse.ArgumentParser(
        description="uvbox end-to-end tests",
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    parser.add_argument(
        "mode",
        nargs="?",
        choices=["pypi", "wheel", "all"],
        default="all",
        help="which flow to run (default: all)",
    )
    args = parser.parse_args()

    go_os, go_arch = detect_native_platform()

    if args.mode == "pypi":
        return 0 if run_pypi(go_os, go_arch) else 1
    if args.mode == "wheel":
        return 0 if run_wheel(go_os, go_arch) else 1

    # "all": run both; exit nonzero if either fails.
    pypi_ok = run_pypi(go_os, go_arch)
    wheel_ok = run_wheel(go_os, go_arch)
    return 0 if (pypi_ok and wheel_ok) else 1


if __name__ == "__main__":
    sys.exit(main())
