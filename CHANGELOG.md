# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [1.0.0] - 2026-10-06

First stable release — the project is back on track.

### Fixed

- **Installers produced by the packer no longer fail with "GPKLZ4 magic not found".**
  The extractor expected an LZ4-compressed payload with a footer, while the
  packer still appended a raw zip. The format is now implemented once in the
  new `internal/bundle` package shared by both binaries.
- `DisplayIcon` in the registry now points at the configured main executable
  instead of assuming `<AppName>.exe`.
- The uninstaller no longer panics on `filepath.Walk` errors and tolerates
  files it cannot remove.
- Payload extraction now rejects absolute paths and `..` traversal (zip-slip).

### Added

- JSON config file support in the packer (`-config installer.json`), as
  always promised by the README. CLI flags override config values.
- Optional `publisher` field, written to the registry uninstall entry.
- Input validation: the packer fails fast on missing metadata, a missing
  source directory, or a main executable not present in the payload.
- Installer GUI now shows failures in the window instead of dying silently,
  and writes `install_log.txt` next to the binary for diagnostics.
- Unit tests for the bundle format, metadata, and safe extraction.
- CI workflow (`go.yml`): vet, tests, and builds on Ubuntu and Windows;
  tagged pushes automatically build cross-platform binaries and publish a
  GitHub release with checksums.
- `Makefile`, `.gitignore`, and an example config under `examples/`.

### Changed

- Module path is now `github.com/omnizs38/gopack`, so
  `go install github.com/omnizs38/gopack/cmd/packer@latest` works.
- Error handling throughout: previously ignored errors are now propagated.
- Removed the accidentally committed `install_log.txt` debug log.

[1.0.0]: https://github.com/omnizs38/gopack/releases/tag/v1.0.0
