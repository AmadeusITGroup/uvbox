<div align="center">

# UVBOX

### **Fast, simple and cross-platform Python application packaging**

*Package your Python apps as self-boostrapping executables for every platforms.*

[![Go](https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white)](https://go.dev/)
[![Python](https://img.shields.io/badge/Python-FFD43B?style=for-the-badge&logo=python&logoColor=blue)](https://www.python.org/)

[![Linux](https://img.shields.io/badge/Linux-FCC624?style=for-the-badge&logo=linux&logoColor=black)](https://www.kernel.org/)
[![MacOS](https://img.shields.io/badge/mac%20os-000000?style=for-the-badge&logo=apple&logoColor=white)](https://www.apple.com/os/macos/)
[![Windows](https://img.shields.io/badge/Windows-0078D6?style=for-the-badge&logo=windows&logoColor=white)](https://www.microsoft.com/windows)

[Features](#features) •
[Comparison](#comparison) •
[Quick Start](#quick-start) •
[Configuration](#configuration) •
[How It Works](#how-it-works) •
[Advanced Usage](#advanced-usage) •
[Examples](#examples)

</div>

---

## What is uvbox?

**`uvbox`** generates standalone executables that bootstrap an embedded [uv](https://github.com/astral-sh/uv) installation to automatically setup and run your Python application in a fully isolated environment — for any platform, from any platform.

**Python is not required on the target system to run your application.** The binary handles everything: downloads the right Python version, creates a virtual environment, installs dependencies, and runs your application.

<div align="center">
  <img src="./assets/demo.gif" alt="uvbox demo">
</div>

## Features

- **Package from PyPI or Wheels** — Install your application from package indexes or choose to bundle local wheel files
- **True Cross-Compilation** — Build binaries for Linux, macOS, and Windows (AMD64/ARM64) from any platform in seconds
- **Auto-Updates** — Built-in version checking and self-update/fallback capabilities for your binaries
- **Dependency Freezing** — Use constraints files to ensure reproducible installations
- **Enterprise-Friendly** — Support for custom registries, mirrors, and CA certificates
- **Fast** — Powered by Go, builds complete in very few seconds
- **Simple Integration** — Add as a dev dependency to your Python project

## Comparison

| Feature | uvbox | pyapp | PyInstaller |
|---------|-------|-------|-------------|
| Build Time | ~1s | ~30s | ~1-5min |
| Cross-Compilation | ✅ All platforms from any platform | ⚠️ Requires target toolchains | ❌ Native only |
| macOS from Linux/Windows | ✅ Out of the box | ❌ Forbidden by Apple license | ❌ Forbidden by Apple license |
| Updates | ✅ Built-in | ✅ Built-in | ❌ Manual |
| Fallbacks | ✅ Version fallback support | ❌ Not supported | ❌ Not supported |
| Distribution | ⚠️ Downloads at first run | ⚠️ Downloads at first run | ✅ Bundles everything (offline-ready) |

**🎯 Choosing the right tool:**

**uvbox** excels at fast, cross-platform builds with minimal setup, built-in automatic updates, and version fallback mechanisms. It downloads dependencies at first run, making binaries small but requiring internet connectivity initially.

**PyInstaller** bundles everything into the binary, creating larger files but ensuring complete offline functionality and maximum stability (no runtime network dependencies). However, it requires native builds per platform and lacks built-in update mechanisms.

**💡 Use uvbox when:** You want fast builds, easy cross-compilation, or when enforced updates/fallbacks may be required, and don't mind first-run downloads.

**💡 Use PyInstaller when:** You need guaranteed offline functionality, distribute in air-gapped environments, or only target a single platform (especially Linux-only deployments).

## Quick Start

### Installation

Add uvbox to your project from [PyPI](https://pypi.org/project/uvbox):

```bash
# With uv
uv add --dev uvbox

# With pip
pip install uvbox
```

> [!NOTE]
> Installing from PyPI automatically includes **Go** and **nfpm** as dependencies, so you don't need to install them separately.

### Basic Usage

Create a simple configuration and build:

```bash
# Create a minimal config file
cat > uvbox.toml <<EOF
[package]
name = "cowsay"
script = "cowsay"
EOF

# Build for all platforms
uvbox pypi --config uvbox.toml

# Output:
# ✓ DARWIN/AMD64 → cowsay-x86_64-apple-darwin.tar.gz
# ✓ DARWIN/ARM64 → cowsay-aarch64-apple-darwin.tar.gz
# ✓ LINUX/AMD64 → cowsay-x86_64-unknown-linux-gnu.tar.gz
# ✓ LINUX/ARM64 → cowsay-aarch64-unknown-linux-gnu.tar.gz
# ✓ WINDOWS/AMD64 → cowsay-x86_64-pc-windows-msvc.zip
```

Your executables are now in the `dist/` directory!

### Build from Wheels

Package local wheel files instead of installing from PyPI:

```bash
uvbox wheel --config uvbox.toml ./my-app.whl
```

## Configuration

### Using pyproject.toml

Embed configuration directly in your Python project:

```toml
[tool.uvbox.package]
name = "my-awesome-app"
script = "main"  # Entry point from your package

[tool.uvbox.package.version]
dynamic = "https://example.com/my-app/version.txt"  # Fetch version from URL
static = "1.0.0"  # Fallback version if dynamic fetch fails
auto-update = true

# Optional: Constrain dependencies
[tool.uvbox.package.constraints]
dynamic = "https://example.com/my-app/<VERSION>/constraints.txt"

# uv index and mirror configuration will be automatically be used
[[tool.uv.index]]
name = "company-pypi"
url = "https://my.artifactory.com/artifactory/api/pypi/pypi-mirror/simple"
default = true

[tool.uv]
python-install-mirror = "https://my.github.remote/astral-sh/python-build-standalone/releases/download"
```

Then build directly from your project directory:

```bash
# uvbox automatically detects pyproject.toml
uvbox pypi
```

### Using Standalone Config Files

Create a dedicated `uvbox.toml` file:

```toml
[package]
name = "my-app"
script = "my_entrypoint"

[package.version]
# Pin to specific version
static = "2.1.0"
# Or use dynamic version from URL
dynamic = "https://example.com/my-app/version.txt"
# Enable automatic updates before each run
auto-update = true

[package.constraints]
# Freeze dependencies with a constraints file
# <VERSION> placeholder is automatically replaced
dynamic = "https://example.com/my-app/<VERSION>/constraints.txt"

# Or use a static constraints file
static = "./constraints.txt"

[certificates]
# Bundle custom CA certificates (useful behind corporate firewalls)
path = "./ca-bundle.crt"

[uv]
# Optional: Specify which uv version to use
version = "0.11.6"
# Optional: Use a mirror for downloading uv itself
mirror = "https://mirror.example.com/uv/releases/download"
# Configure uv runtime behavior with environment variables
environment = [
    "UV_INDEX_URL=https://pypi.example.com/simple",
    "UV_PYTHON=3.12",
    "UV_PYTHON_INSTALL_MIRROR=https://mirror.example.com/python-builds"
]
```

### Configuration Reference

#### `[package]`
Core package configuration.

- **`name`** (required) — Package name to install from PyPI
- **`script`** (required) — Entry point script to run (from `[project.scripts]` in your package)

#### `[package.version]`
Version management and updates.

- **`static`** — Pin to a specific version (e.g., `"1.2.3"`)
- **`dynamic`** — URL to a text file containing the version to install
- **`auto-update`** — Check for updates before each run (may slow startup)

#### `[package.constraints]`
Dependency freezing for reproducible installs.

- **`static`** — Path to local constraints file
- **`dynamic`** — URL to remote constraints file (supports `<VERSION>` placeholder)

#### `[certificates]`
Bundle custom CA certificates.

- **`path`** — Path to certificate bundle (relative to working directory)

The binary automatically sets `REQUESTS_CA_BUNDLE` and `SSL_CERT_FILE` environment variables.

#### `[uv]`
Configure uv installation and behavior.

- **`version`** — Specific uv version to download and use (e.g., `"0.11.6"`)
- **`mirror`** — Alternative download URL for uv releases (e.g., `"https://mirror.example.com/uv/releases/download"`)
- **`environment`** — Array of environment variables to set for uv runtime behavior

See [uv's documentation](https://docs.astral.sh/uv/configuration/environment/) for available environment variables.

### Config File Discovery

uvbox automatically searches for configuration files:

1. Explicit path via `--config` flag
2. `uvbox.toml` in current directory or parents
3. `pyproject.toml` with `[tool.uvbox]` section in current directory or parents

## How It Works

**uvbox** leverages Go's powerful cross-compilation to create lightweight executables that:

1. **Embed uv** — Each binary contains the uv installation URL and configuration
2. **Bootstrap on First Run** — Downloads and extracts uv to an isolated XDG-compliant directory
3. **Install Your App** — Uses `uv tool install` to set up your package and dependencies in complete isolation
4. **Run** — Executes your application's entry point with the configured environment

All files are stored in XDG-compliant directories (`$XDG_DATA_HOME/uvbox/` or `~/.local/share/uvbox/` by default), ensuring a clean and standardized file system layout.

### Why Go?

- **Fast Compilation** — ~1 second vs ~30 seconds for Rust-based alternatives
- **Cross-compilation out of the box** — Build macOS binaries from Linux/Windows without licenses or complex toolchains
- **No dependencies** — Just set `GOOS` and `GOARCH` environment variables
- **Compiles into static binary** — Just a single executable, with no runtime dependencies

### Runtime Behavior

When a user runs your binary:

```bash
./my-app --help
```

The first run automatically:
1. Extracts embedded uv to `$XDG_DATA_HOME/uvbox/<identifier>/uv/` (defaults to `~/.local/share/uvbox/<identifier>/uv/`)
2. Downloads Python if needed to `$XDG_DATA_HOME/uvbox/<identifier>/python/` (defaults to `~/.local/share/uvbox/<identifier>/python/`)
3. Installs your package in an isolated environment at `$XDG_DATA_HOME/uvbox/<identifier>/tools/` (defaults to `~/.local/share/uvbox/<identifier>/tools/`)
4. Runs your script

Subsequent runs skip straight to execution (unless updates are configured).

> [!NOTE]
> The `<identifier>` is a hash computed from your package name, script, and configuration to ensure complete isolation between different applications.

### Built-in Commands

Every generated binary includes these `self` commands:

- `<app> self update` — Update the package to the latest available version
- `<app> self remove` — Remove installation (clean up installed files and virtual environment)
- `<app> self path` — Display paths related to the installation
- `<app> self cache` — Manage cache
- `<app> self uv` — Run installation uv executable inside the isolated environment

## Advanced Usage

### Linux Package Generation

Generate `.deb` and `.rpm` packages alongside binaries:

```bash
# Create nfpm.yaml configuration
cat > nfpm.yaml <<EOF
name: my-app
maintainer: "Your Name <you@example.com>"
description: "My awesome application"
homepage: "https://github.com/you/my-app"
license: "MIT"

# Template variables
arch: ${UVBOX_ARCH}
platform: ${UVBOX_PLATFORM}
version: ${UVBOX_VERSION}

# Copy executable to target folder
contents:
  - src: ${UVBOX_BUILT_EXECUTABLE}
    dst: /usr/bin/my-app

# Built-in pre-remove script that ensures application data are removed
scripts:
  preremove: ./pre_remove.sh
EOF

# Build with packaging
uvbox pypi --nfpm nfpm.yaml --release-version 1.0.0
```

**Available Template Variables:**

uvbox provides environment variables for use in your nfpm configuration:

- **`UVBOX_BUILT_EXECUTABLE`** — Absolute path to the compiled executable
- **`UVBOX_NAME`** — Executable name (from `script` configuration)
- **`UVBOX_PLATFORM`** — Target platform (`linux`, `darwin`, `windows`)
- **`UVBOX_ARCH`** — Target architecture (`amd64`, `arm64`)
- **`UVBOX_VERSION`** — Version from `--release-version` flag

Use them with `${VARIABLE_NAME}` syntax in your nfpm.yaml.

> [!TIP]
> The `preremove: ./pre_remove.sh` script is optional. If you include it in your nfpm configuration, uvbox automatically generates it to call `<executable> self remove`, which cleans up application data (virtual environments, cached Python installations) when the package is uninstalled.

### Custom Registry Example

```toml
[package]
name = "internal-tool"
script = "tool"

[uv]
environment = [
    "UV_INDEX_URL=https://artifactory.company.com/api/pypi/pypi-mirror/simple",
    "UV_EXTRA_INDEX_URL=https://artifactory.company.com/api/pypi/internal-packages/simple",
    "UV_PYTHON_INSTALL_MIRROR=https://internal-mirror.company.com/python-builds"
]

[certificates]
path = "./company-ca-bundle.crt"
```

### Version Fallback Strategy

Use dynamic versioning with fallback:

```toml
[package.version]
# Try to fetch latest version from URL
dynamic = "https://cdn.example.com/my-app/latest.txt"
# Fallback to this version if URL is unreachable
static = "1.0.0"
# Check for updates before each run
auto-update = true
```

## Examples

See the [`examples/`](./examples) directory for complete working examples:

- [`simple-app.toml`](./examples/pypi/simple-app.toml) — Minimal PyPI package
- [`custom-registry.toml`](./examples/pypi/custom-registry.toml) — Custom registry and mirrors
- [`custom-certs.toml`](./examples/pypi/custom-certs.toml) — Corporate CA bundle
- [`optional-dependency.toml`](./examples/pypi/optional-dependency.toml) - Install a package with an optional dependency

## Requirements

### Build Time

- **Go** 1.21+ (automatically fetched if installed via PyPI package)
- **nfpm** (optional, only if generating `.deb`/`.rpm` packages)

### Runtime (Generated Binaries)

- **libc** (standard C library, required by Python itself)

## License

[MIT](LICENSE) © 2025 Amadeus

## Contributing

We welcome contributions!

## Acknowledgments

Heavily inspired by the excellent [pyapp](https://github.com/ofek/pyapp) project. Built with:

- [uv](https://github.com/astral-sh/uv) — Blazing fast Python package installer
- [Go](https://go.dev/) — Enabling true cross-compilation magic
- [pyapp](https://github.com/ofek/pyapp) — Inspiration and reference implementation
