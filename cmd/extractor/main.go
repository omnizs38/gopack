//go:build windows

// Command extractor is the installer stub. It is distributed as a template
// binary; the packer appends an LZ4-compressed payload and a footer to it.
// The same binary doubles as the uninstaller when copied to uninstall.exe
// or invoked with --uninstall.
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"image/color"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/omnizs38/gopack/internal/bundle"
	"github.com/omnizs38/gopack/internal/install"
	"github.com/omnizs38/gopack/internal/metadata"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/sys/windows/registry"
)

type installState int

const (
	stateIdle installState = iota
	stateInstalling
	stateDone
	stateFailed
)

var ui struct {
	sync.Mutex
	state    installState
	progress float32
	status   string
}

func setStatus(state installState, progress float32, status string) {
	ui.Lock()
	ui.state, ui.progress, ui.status = state, progress, status
	ui.Unlock()
}

func main() {
	// GUI apps have no console; keep a log file next to the binary so
	// failures are diagnosable.
	if exePath, err := os.Executable(); err == nil {
		logPath := filepath.Join(filepath.Dir(exePath), "install_log.txt")
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
			defer f.Close()
			log.SetOutput(f)
		}
	}

	uninstallMode := flag.Bool("uninstall", false, "Run uninstaller")
	flag.Parse()

	exeName := filepath.Base(os.Args[0])
	if *uninstallMode || exeName == "uninstall.exe" {
		if err := runUninstaller(); err != nil {
			log.Printf("uninstall failed: %v", err)
			os.Exit(1)
		}
		return
	}

	go runInstallerGUI()
	app.Main()
}

// loadPayload reads the embedded metadata and archive from the running exe.
func loadPayload() (metadata.Metadata, *zip.Reader, error) {
	exePath, err := os.Executable()
	if err != nil {
		return metadata.Metadata{}, nil, fmt.Errorf("locating executable: %w", err)
	}
	file, err := os.Open(exePath)
	if err != nil {
		return metadata.Metadata{}, nil, fmt.Errorf("opening executable: %w", err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return metadata.Metadata{}, nil, err
	}

	zipReader, err := bundle.Read(file, info.Size())
	if err != nil {
		return metadata.Metadata{}, nil, err
	}

	var meta metadata.Metadata
	for _, f := range zipReader.File {
		if f.Name == metadata.MetadataFile {
			rc, err := f.Open()
			if err != nil {
				return meta, nil, err
			}
			defer rc.Close()
			var data []byte
			data, err = io.ReadAll(rc)
			if err != nil {
				return meta, nil, err
			}
			meta, err = metadata.Decode(data)
			if err != nil {
				return meta, nil, fmt.Errorf("parsing metadata: %w", err)
			}
			return meta, zipReader, meta.Validate()
		}
	}
	return meta, nil, fmt.Errorf("%s not found in payload", metadata.MetadataFile)
}

func runInstallerGUI() {
	meta, zipReader, err := loadPayload()
	if err != nil {
		// No payload: someone ran the bare template. Not fatal for the log,
		// but there is nothing to install.
		log.Printf("error reading embedded data: %v", err)
		meta = metadata.Metadata{AppName: "gopack"}
		setStatus(stateFailed, 0, "This installer is corrupt or incomplete.")
	} else {
		setStatus(stateIdle, 0, "Ready to install "+meta.AppName+" "+meta.AppVersion)
	}

	w := new(app.Window)
	w.Option(app.Title(meta.AppName+" Setup"), app.Size(unit.Dp(420), unit.Dp(160)))

	th := material.NewTheme()
	var ops op.Ops
	installBtn := new(widget.Clickable)

	for {
		e := w.Event()
		switch e := e.(type) {
		case app.FrameEvent:
			gtx := app.NewContext(&ops, e)

			ui.Lock()
			state := ui.state
			ui.Unlock()

			if installBtn.Clicked(gtx) {
				switch state {
				case stateIdle:
					setStatus(stateInstalling, 0, "Preparing...")
					go performInstallation(meta, zipReader, w)
				case stateDone, stateFailed:
					os.Exit(0)
				}
			}

			layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceAround}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					ui.Lock()
					defer ui.Unlock()
					lbl := material.H6(th, ui.status)
					lbl.Alignment = text.Middle
					return lbl.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if state == stateInstalling {
						ui.Lock()
						p := ui.progress
						ui.Unlock()
						return material.ProgressBar(th, p).Layout(gtx)
					}
					return layout.Dimensions{}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					var btnText string
					switch state {
					case stateInstalling:
						btnText = "Installing..."
					case stateDone:
						btnText = "Close"
					case stateFailed:
						btnText = "Exit"
					default:
						btnText = "Install"
					}
					btn := material.Button(th, installBtn, btnText)
					if state == stateInstalling {
						btn.Background = color.NRGBA{R: 200, G: 200, B: 200, A: 255}
					}
					return btn.Layout(gtx)
				}),
			)
			e.Frame(gtx.Ops)

		case app.DestroyEvent:
			os.Exit(0)
		}
	}
}

func installDir(meta metadata.Metadata) string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", meta.AppName)
}

func performInstallation(meta metadata.Metadata, zipReader *zip.Reader, w *app.Window) {
	fail := func(err error) {
		log.Printf("installation failed: %v", err)
		setStatus(stateFailed, 0, "Installation failed. See install_log.txt.")
		w.Invalidate()
	}

	dir := installDir(meta)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fail(fmt.Errorf("creating install directory: %w", err))
		return
	}

	err := install.ExtractAll(zipReader, dir, func(name string, index, total int) {
		setStatus(stateInstalling, float32(index)/float32(total),
			fmt.Sprintf("Extracting: %s (%d/%d)", name, index, total))
		w.Invalidate()
	})
	if err != nil {
		fail(err)
		return
	}

	// The uninstaller is a copy of this same binary; --uninstall switches mode.
	exePath, err := os.Executable()
	if err != nil {
		fail(err)
		return
	}
	uninstallPath := filepath.Join(dir, "uninstall.exe")
	if err := copyFile(exePath, uninstallPath); err != nil {
		fail(fmt.Errorf("installing uninstaller: %w", err))
		return
	}

	mainExePath := filepath.Join(dir, filepath.FromSlash(meta.MainExe))
	if err := createShortcut(meta.AppName, mainExePath); err != nil {
		log.Printf("warning: shortcut not created: %v", err)
	}
	if err := registerUninstaller(meta, dir, uninstallPath, mainExePath); err != nil {
		log.Printf("warning: uninstaller not registered: %v", err)
	}

	log.Printf("installed %s %s to %s", meta.AppName, meta.AppVersion, dir)
	setStatus(stateDone, 1, "Installation completed successfully!")
	w.Invalidate()
}

func runUninstaller() error {
	meta, _, err := loadPayload()
	if err != nil {
		return fmt.Errorf("reading metadata: %w", err)
	}
	dir := installDir(meta)
	uninstallPath := filepath.Join(dir, "uninstall.exe")

	_ = registry.DeleteKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Uninstall\`+meta.AppName)
	_ = os.Remove(filepath.Join(os.Getenv("USERPROFILE"), "Desktop", meta.AppName+".lnk"))

	// Remove everything except the running uninstaller itself.
	walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if path != uninstallPath && !info.IsDir() {
			if err := os.Remove(path); err != nil {
				log.Printf("warning: could not remove %s: %v", path, err)
			}
		}
		return nil
	})
	if walkErr != nil && !os.IsNotExist(walkErr) {
		log.Printf("warning: cleanup incomplete: %v", walkErr)
	}

	// Self-delete after a short delay, then remove the install tree.
	cmdStr := fmt.Sprintf(`timeout /t 2 > nul & del /f /q "%s" & rmdir /s /q "%s"`, uninstallPath, dir)
	cmd := exec.Command("cmd", "/c", cmdStr)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("scheduling self-delete: %w", err)
	}
	return nil
}

func copyFile(src, dst string) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()
	_, err = io.Copy(out, in)
	return err
}

func createShortcut(appName, targetExe string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	shortcutPath := filepath.Join(home, "Desktop", appName+".lnk")
	psScript := fmt.Sprintf(
		`$ws = New-Object -ComObject WScript.Shell; $sc = $ws.CreateShortcut("%s"); $sc.TargetPath = "%s"; $sc.WorkingDirectory = "%s"; $sc.Save()`,
		shortcutPath, targetExe, filepath.Dir(targetExe),
	)
	out, err := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", psScript).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}
	return nil
}

func registerUninstaller(meta metadata.Metadata, dir, uninstallPath, mainExePath string) error {
	keyPath := `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + meta.AppName
	k, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	values := map[string]string{
		"DisplayName":     meta.AppName,
		"DisplayVersion":  meta.AppVersion,
		"InstallLocation": dir,
		"UninstallString": `"` + uninstallPath + `" --uninstall`,
		"DisplayIcon":     mainExePath,
	}
	if meta.Publisher != "" {
		values["Publisher"] = meta.Publisher
	}
	for name, value := range values {
		if err := k.SetStringValue(name, value); err != nil {
			return fmt.Errorf("setting %s: %w", name, err)
		}
	}
	return nil
}
