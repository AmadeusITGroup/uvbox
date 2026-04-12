#!/usr/bin/env python3
"""
uvbox end-to-end test

Builds boxer from source, cross-compiles the cowsay example for all
platform/arch targets, extracts the native binary, and runs it end-to-end.

Requirements: Go toolchain (managed by mise). No Python dependencies.
"""

import os
import platform
import shutil
import subprocess
import sys
import tarfile
import tempfile
import time
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

TOTAL_STEPS = 5


# ── Display ──────────────────────────────────────────────────────────────────


class Display:
    """Minimal terminal display with optional ANSI colors."""

    WIDTH = 56

    def __init__(self):
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
        print(f"\n  {self.cyan(f'[{number}/{TOTAL_STEPS}]')} {self.bold(text)}")

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


# ── Steps ────────────────────────────────────────────────────────────────────


def step_build_boxer(d: Display, work_dir: Path) -> Path:
    """Build the boxer (uvbox) binary from source.

    Order matters: box/ must be generated first because boxer's generate step
    bundles the box/ directory into an embedded archive.
    """
    d.step(1, "Building boxer")

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


def step_cross_compile(d: Display, boxer_path: Path, work_dir: Path) -> Path:
    """Use the freshly built boxer to cross-compile the cowsay example for all 6 targets.

    A single invocation of `uvbox pypi` produces archives for every OS/arch combination
    (linux, darwin, windows x amd64, arm64). This validates that cross-compilation works
    from the current host.
    """
    d.step(2, "Cross-compiling cowsay for all targets")

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


def _archive_members(archive: Path) -> list[str]:
    """Return the list of filenames inside an archive."""
    if archive.name.endswith(".tar.gz"):
        with tarfile.open(archive, "r:gz") as tf:
            return tf.getnames()
    elif archive.name.endswith(".zip"):
        with zipfile.ZipFile(archive) as zf:
            return zf.namelist()
    return []


def step_verify_archives(d: Display, archive_dir: Path) -> None:
    """Check that all 6 platform/arch archives exist and contain the expected executable."""
    d.step(3, "Verifying archives")

    for go_os, go_arch, suffix in TARGETS:
        archive = archive_dir / f"{EXECUTABLE_NAME}-{suffix}"
        exe_name = f"{EXECUTABLE_NAME}.exe" if go_os == "windows" else EXECUTABLE_NAME

        with d.task(f"{go_os}/{go_arch}") as detail:
            if not archive.exists():
                raise FileNotFoundError(f"not found: {archive.name}")

            # Verify the archive contains the expected executable
            members = _archive_members(archive)
            if exe_name not in members:
                raise RuntimeError(
                    f"executable '{exe_name}' not found in {archive.name}, "
                    f"contents: {members}"
                )

            detail.append(f"({format_size(archive.stat().st_size)})")


def step_extract_native(
    d: Display, archive_dir: Path, work_dir: Path, go_os: str, go_arch: str
) -> Path:
    """Extract the archive matching the current host platform.

    Only the native-platform archive can actually be executed, so this is the
    one we unpack for the end-to-end run in the next step.
    Uses tarfile for .tar.gz (linux/macOS) and zipfile for .zip (Windows).
    """
    d.step(4, f"Extracting native binary ({go_os}/{go_arch})")

    suffix = next(s for o, a, s in TARGETS if o == go_os and a == go_arch)
    archive = archive_dir / f"{EXECUTABLE_NAME}-{suffix}"
    extract_dir = work_dir / "run"
    extract_dir.mkdir(exist_ok=True)

    with d.task(archive.name) as detail:
        if archive.name.endswith(".tar.gz"):
            with tarfile.open(archive, "r:gz") as tf:
                tf.extractall(extract_dir, filter="data")
        elif archive.name.endswith(".zip"):
            with zipfile.ZipFile(archive) as zf:
                zf.extractall(extract_dir)
        else:
            raise RuntimeError(f"unknown archive format: {archive.name}")
        detail.append("")

    exe_name = f"{EXECUTABLE_NAME}.exe" if go_os == "windows" else EXECUTABLE_NAME
    exe_path = extract_dir / exe_name

    if not exe_path.exists():
        raise FileNotFoundError(f"Executable not found after extraction: {exe_path}")

    if go_os != "windows":
        exe_path.chmod(0o755)

    return exe_path


def step_run_e2e(d: Display, exe_path: Path) -> None:
    """Run the extracted binary end-to-end.

    This exercises the full runtime chain: the binary bootstraps uv, uses it to
    install cowsay from PyPI into an isolated environment, then invokes cowsay.
    We verify both a zero exit code and that the expected output appears.
    """
    d.step(5, "Running end-to-end")

    test_phrase = "e2e-test"

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


# ── Main ─────────────────────────────────────────────────────────────────────


def main() -> int:
    d = Display()
    go_os, go_arch = detect_native_platform()

    d.header("uvbox end-to-end test", f"Platform: {go_os}/{go_arch}")

    work_dir = Path(tempfile.mkdtemp(prefix="uvbox-e2e-"))
    success = False

    try:
        boxer_path = step_build_boxer(d, work_dir)
        archive_dir = step_cross_compile(d, boxer_path, work_dir)
        step_verify_archives(d, archive_dir)
        exe_path = step_extract_native(d, archive_dir, work_dir, go_os, go_arch)
        step_run_e2e(d, exe_path)
        success = True
    except Exception as exc:
        if not isinstance(
            exc, (FileNotFoundError, RuntimeError, subprocess.CalledProcessError)
        ):
            print(f"\n        Error: {exc}")
    finally:
        shutil.rmtree(work_dir, ignore_errors=True)

    d.result(success)
    return 0 if success else 1


if __name__ == "__main__":
    sys.exit(main())
