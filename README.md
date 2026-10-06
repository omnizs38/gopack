# gopack

![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/omnizs38/gopack/go.yml?branch=main)
![GitHub License](https://img.shields.io/github/license/omnizs38/gopack)
![GitHub Top Language](https://img.shields.io/github/languages/top/omnizs38/gopack)
![Go Version](https://img.shields.io/github/go-mod/go-version/omnizs38/gopack)
![Latest Release](https://img.shields.io/github/v/release/omnizs38/gopack)

A minimalist, high-performance Windows installer builder written in Go. Uses binary fusion to merge executable logic with an app payload into a single, UAC-free `setup.exe`.

No complex Pascal or XML scripting required — configure everything with a few lines of JSON. Features auto-uninstall generation, desktop shortcuts, and seamless registry cleanup.

---

### Key Features

* **Simple JSON Config** — Describe your installer in a few lines of JSON.
* **UAC-Free by Default** — Installs to `%LOCALAPPDATA%\Programs` without admin privileges.
* **Binary Fusion** — Bundles your app files directly inside a single executable (LZ4-compressed).
* **Clean Uninstaller** — Automatically registers in Windows Settings / Control Panel for clean removal.
* **Safe Extraction** — Payload paths are validated against directory traversal.

### How It Works

1. `gopack` (the packer) takes your app folder and an **extractor template** — a small Windows GUI stub.
2. It compresses your files into an LZ4-compressed archive and appends it to the stub, producing one `setup.exe`.
3. When a user runs `setup.exe`, the extractor shows a minimal install window, unpacks the app to `%LOCALAPPDATA%\Programs\<AppName>`, creates a desktop shortcut, and registers an uninstaller.

### Install

```sh
go install github.com/omnizs38/gopack/cmd/packer@latest
```

Or grab a prebuilt `gopack` binary and `extractor_template.exe` from the [latest release](https://github.com/omnizs38/gopack/releases/latest).

### Usage

Create a config file (see [`examples/installer.json`](examples/installer.json)):

```json
{
  "src": "./myapp",
  "out": "setup.exe",
  "name": "MyApp",
  "version": "1.0.0",
  "exe": "myapp.exe",
  "publisher": "Your Name",
  "template": "./extractor_template.exe"
}
```

Build the installer:

```sh
gopack -config installer.json
```

Every field can also be set (or overridden) with a CLI flag — run `gopack -h` for the full list. Flags take precedence over the config file.

### Building the Extractor Template

The extractor stub is Windows-only, but cross-compiles from any OS (no cgo):

```sh
make extractor   # produces build/extractor_template.exe
```

### Development

```sh
make test             # run unit tests
make vet              # go vet
make release-binaries # cross-compile everything that ships in a release
```

The payload format (`[template][LZ4(zip)][8-byte size]["GPKLZ4"]`) is implemented once in `internal/bundle` and shared by both the packer and the extractor, so the two sides cannot drift apart.

### License

MIT
