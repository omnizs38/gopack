# gopack

![GitHub Workflow Status](https://shields.io)
![GitHub License](https://shields.io)
![GitHub Top Language](https://shields.io)

A minimalist, high-performance Windows installer builder written in Go. Uses binary fusion to merge executable logic with app payload into a single, UAC-free `setup.exe`. 

No complex Pascal or XML scripting required—configure everything instantly via JSON. Features auto-uninstall generation, desktop shortcuts, and seamless registry cleanup. 

---

### Key Features
* **Simple JSON Config** — Describe your installer in a few lines of JSON.
* **UAC-Free by Default** — Installs to local folders without admin privileges.
* **Binary Fusion** — Bundles your app files directly inside a single executable.
* **Clean Uninstaller** — Automatically registers in Windows Control Panel for clean removal.
