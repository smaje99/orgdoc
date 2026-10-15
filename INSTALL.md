# OrgDoc Runtime installation

This file ships inside each release archive. Supported release targets are Linux (`amd64`, `arm64`) and macOS (`amd64`, `arm64`). Windows is experimental and is not published as an official release target.

## Verify and install

Releases use portable `.tar.gz` archives for Linux and macOS (`amd64` and `arm64`). The core package is a standalone executable; Python workers and their system dependencies are installed separately when needed. Native package-manager installers such as `.deb`, `.rpm`, and `.pkg` are not provided in this release format.

Download the archive for your operating system and architecture, and `SHA256SUMS` from the same GitHub release. Verify the selected archive before extracting it:

```sh
ARCHIVE=orgdoc-0.1.0-linux-amd64.tar.gz
grep " ${ARCHIVE}$" SHA256SUMS | sha256sum -c -
```

On macOS, use `grep " ${ARCHIVE}$" SHA256SUMS | shasum -a 256 -c -` instead. Replace `ARCHIVE` with the filename for your target. Extract the archive and install `orgdoc` to a user-owned directory:

```sh
tar -xzf orgdoc-0.1.0-linux-amd64.tar.gz
mkdir -p "$HOME/.local/bin"
install -m 755 orgdoc "$HOME/.local/bin/orgdoc"
```

Replace the archive name for your target. Add `$HOME/.local/bin` to `PATH` if needed, then check `orgdoc --version` and `orgdoc --help`. No administrator privileges are required.

The source checkout also includes `scripts/install-orgdoc.sh VERSION`, which downloads the selected release asset from `smaje99/orgdoc`, verifies its SHA-256 checksum, and installs to `$HOME/.local/bin` by default. Set `INSTALL_DIR` to choose another user-writable directory. Set `ORGDOC_REPOSITORY` to use a mirror, or `ORGDOC_RELEASE_BASE_URL` to install from a local release directory during testing.

The extracted core archive also includes `install-worker.sh`. After installing `uv` and a supported Python version, install only a needed worker with `./install-worker.sh reportlab-invoice 0.1.0`, replacing `0.1.0` with the OrgDoc release version. This verifies the worker bundle checksum, creates an isolated environment under the user data directory, and links its commands into `$HOME/.local/bin`. FFmpeg, Pandoc/TeX, GPU drivers, and ML models remain separate prerequisites and are not installed by this script.

Core archives include the portable agent Skill at `skills/orgdoc/SKILL.md`. Copy it into the project-level or user-level Skills directory documented by the chosen agent. OrgDoc does not choose that directory or install Skills globally; see [agent Skill setup](docs/installation/agents.md).

## Build from source

Install Go 1.27 or later and run:

```sh
scripts/build-release.sh
```

The script cross-compiles the four official Linux and macOS targets with CGO disabled, embeds the version from `VERSION`, creates release archives under `build/release/`, and writes `SHA256SUMS`.

## Runtime and workers

The OrgDoc binary requires no Python runtime, ML dependencies, or network access for help, version, organization validation, and capability discovery. Renderer capabilities are separate programs. Follow each worker's installation guide only when that capability is needed. The Python `media-report` compatibility command remains a separate distribution during the migration release.
