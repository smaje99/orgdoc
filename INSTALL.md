# OrgDoc Runtime installation

This file ships inside each release archive. Supported release targets are Linux (`amd64`, `arm64`) and macOS (`amd64`, `arm64`). Windows is experimental and is not published as an official release target.

## Verify and install

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

The source checkout also includes `scripts/install-orgdoc.sh VERSION`, which downloads the selected release asset, verifies its SHA-256 checksum, and installs to `$HOME/.local/bin` by default. Set `INSTALL_DIR` to choose another user-writable directory.

## Build from source

Install Go 1.27 or later and run:

```sh
scripts/build-release.sh
```

The script cross-compiles the four official Linux and macOS targets with CGO disabled, embeds the version from `VERSION`, creates release archives under `build/release/`, and writes `SHA256SUMS`.

## Runtime and workers

The OrgDoc binary requires no Python runtime, ML dependencies, or network access for help, version, organization validation, and capability discovery. Renderer capabilities are separate programs. Follow each worker's installation guide only when that capability is needed. The Python `media-report` compatibility command remains a separate distribution during the migration release.
