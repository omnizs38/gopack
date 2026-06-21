# gopack

![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/omnizs38/gopack/go.yml?branch=main)
![GitHub License](https://img.shields.io/github/license/omnizs38/gopack)
![GitHub Top Language](https://img.shields.io/github/languages/top/omnizs38/gopack)
![Go Version](https://img.shields.io/github/go-mod/go-version/omnizs38/gopack)
![Latest Release](https://img.shields.io/github/v/release/omnizs38/gopack)

A minimalist, high-performance Windows installer builder written in Go. Uses binary fusion to merge executable logic with app payload into a single, UAC-free `setup.exe`. 

No complex Pascal or XML scripting required—configure everything instantly via JSON. Features auto-uninstall generation, desktop shortcuts, and seamless registry cleanup. 

---

### Key Features
* **Simple JSON Config** — Describe your installer in a few lines of JSON.
* **UAC-Free by Default** — Installs to local folders without admin privileges.
* **Binary Fusion** — Bundles your app files directly inside a single executable.
* **Clean Uninstaller** — Automatically registers in Windows Control Panel for clean removal.
