package main

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/json"
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

	"gopack/internal/metadata"

	"gioui.org/app"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/text"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/pierrec/lz4/v4"
	"golang.org/x/sys/windows/registry"
)

type installState int

const (
    StateIdle installState = iota
    StateInstalling
    StateDone
)

var (
    currentState installState = StateIdle
    progressValue float32
    statusText    string = "Ready to install"
    uiMutex       sync.Mutex
)

func main() {
    uninstallMode := flag.Bool("uninstall", false, "Run uninstaller")
    flag.Parse()

    exeName := filepath.Base(os.Args[0])
    if *uninstallMode || exeName == "uninstall.exe" {
        runUninstaller()
        return
    }

    go runInstallerGUI()
    app.Main()
}

func getEmbeddedData() (metadata.Metadata, *zip.Reader, error) {
    var meta metadata.Metadata
    exePath, _ := os.Executable()
    file, _ := os.Open(exePath)
    defer file.Close()
    info, _ := file.Stat()

    footerSize := int64(14)
    footer := make([]byte, footerSize)
    _, err := file.ReadAt(footer, info.Size()-footerSize)
    if err != nil || string(footer[8:14]) != "GPKLZ4" {
        return meta, nil, fmt.Errorf("GPKLZ4 magic not found")
    }

    lz4Size := int64(binary.LittleEndian.Uint64(footer[:8]))
    lz4Start := info.Size() - footerSize - lz4Size

    lz4Data := make([]byte, lz4Size)
    _, err = file.ReadAt(lz4Data, lz4Start)
    if err != nil {
        return meta, nil, err
    }

    zr := lz4.NewReader(bytes.NewReader(lz4Data))
    var zipBuf bytes.Buffer
    io.Copy(&zipBuf, zr)

    zipReader, err := zip.NewReader(bytes.NewReader(zipBuf.Bytes()), int64(zipBuf.Len()))
    if err != nil {
        return meta, nil, err
    }

    for _, f := range zipReader.File {
        if f.Name == "__metadata.json" {
            rc, _ := f.Open()
            json.NewDecoder(rc).Decode(&meta)
            rc.Close()
            break
        }
    }
    return meta, zipReader, nil
}

func runInstallerGUI() {
    meta, zipReader, err := getEmbeddedData()
    if err != nil {
        log.Fatal("Error reading embedded data:", err)
    }

    w := new(app.Window)
    w.Option(app.Title(meta.AppName + " Setup"))
    w.Option(app.Size(unit.Dp(400), unit.Dp(150)))
    
    th := material.NewTheme()
    var ops op.Ops

    installBtn := new(widget.Clickable)

    for {
        e := w.Event()
        switch e := e.(type) {
        case app.FrameEvent:
            gtx := app.NewContext(&ops, e)

            if installBtn.Clicked(gtx) {
                switch currentState {
                case StateIdle:
                    currentState = StateInstalling
                    go performInstallation(meta, zipReader, w)
                case StateDone:
                    os.Exit(0)
                }
            }

            layout.Flex{Axis: layout.Vertical, Spacing: layout.SpaceAround}.Layout(gtx,
                layout.Rigid(func(gtx layout.Context) layout.Dimensions {
                    uiMutex.Lock()
                    defer uiMutex.Unlock()
                    lbl := material.H6(th, statusText)
                    lbl.Alignment = text.Middle
                    return lbl.Layout(gtx)
                }),
                layout.Rigid(func(gtx layout.Context) layout.Dimensions {
                    if currentState == StateInstalling {
                        return material.ProgressBar(th, progressValue).Layout(gtx)
                    }
                    return layout.Dimensions{}
                }),
                layout.Rigid(func(gtx layout.Context) layout.Dimensions {
                    var btnText string
                    switch currentState {
                    case StateInstalling:
                        btnText = "Installing..."
                    case StateDone:
                        btnText = "Close"
                    default:
                        btnText = "Install"
                    }

                    btn := material.Button(th, installBtn, btnText)
                    if currentState == StateInstalling {
                        btn.Background = color.NRGBA{R: 200, G: 200, B: 200, A: 255}
                    }
                    return btn.Layout(gtx)
                }),
            )
            e.Frame(gtx.Ops)

        case app.DestroyEvent: // Изменено с system.DestroyEvent
            os.Exit(0)
        }
    }
}

func performInstallation(meta metadata.Metadata, zipReader *zip.Reader, w *app.Window) {
    localAppData := os.Getenv("LOCALAPPDATA")
    installDir := filepath.Join(localAppData, "Programs", meta.AppName)
    os.MkdirAll(installDir, 0755)

    totalFiles := len(zipReader.File)
    for i, f := range zipReader.File {
        uiMutex.Lock()
        statusText = fmt.Sprintf("Extracting: %s (%d/%d)", f.Name, i+1, totalFiles)
        progressValue = float32(i) / float32(totalFiles)
        uiMutex.Unlock()
        w.Invalidate()

        targetPath := filepath.Join(installDir, f.Name)
        if f.FileInfo().IsDir() {
            os.MkdirAll(targetPath, 0755)
            continue
        }
        os.MkdirAll(filepath.Dir(targetPath), 0755)

        outFile, _ := os.Create(targetPath)
        rc, _ := f.Open()
        io.Copy(outFile, rc)
        outFile.Close()
        rc.Close()
    }

    exePath, _ := os.Executable()
    uninstallPath := filepath.Join(installDir, "uninstall.exe")
    copyFile(exePath, uninstallPath)

    createShortcut(meta.AppName, filepath.Join(installDir, meta.MainExe))
    registerUninstaller(meta.AppName, meta.AppVersion, installDir, uninstallPath)

    uiMutex.Lock()
    statusText = "Installation completed successfully!"
    progressValue = 1.0
    currentState = StateDone
    uiMutex.Unlock()
    w.Invalidate()
}

func runUninstaller() {
    meta, _, _ := getEmbeddedData()
    localAppData := os.Getenv("LOCALAPPDATA")
    installDir := filepath.Join(localAppData, "Programs", meta.AppName)
    uninstallPath := filepath.Join(installDir, "uninstall.exe")

    registry.DeleteKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Uninstall\`+meta.AppName)
    desktopShortcut := filepath.Join(os.Getenv("USERPROFILE"), "Desktop", meta.AppName+".lnk")
    os.Remove(desktopShortcut)

    filepath.Walk(installDir, func(path string, info os.FileInfo, err error) error {
        if path != uninstallPath && !info.IsDir() {
            os.Remove(path)
        }
        return nil
    })

    cmdStr := fmt.Sprintf(`timeout /t 2 > nul & del /f /q "%s" & rmdir /s /q "%s"`, uninstallPath, installDir)
    cmd := exec.Command("cmd", "/c", cmdStr)
    cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
    cmd.Start()
    os.Exit(0)
}

func copyFile(src, dst string) {
    in, _ := os.Open(src)
    defer in.Close()
    out, _ := os.Create(dst)
    defer out.Close()
    io.Copy(out, in)
}

func createShortcut(appName, targetExe string) {
    desktopDir, _ := os.UserHomeDir()
    shortcutPath := filepath.Join(desktopDir, "Desktop", appName+".lnk")
    psScript := fmt.Sprintf(
        `$ws = New-Object -ComObject WScript.Shell; $sc = $ws.CreateShortcut("%s"); $sc.TargetPath = "%s"; $sc.WorkingDirectory = "%s"; $sc.Save()`,
        shortcutPath, targetExe, filepath.Dir(targetExe),
    )
    exec.Command("powershell", "-ExecutionPolicy", "Bypass", "-Command", psScript).Run()
}

func registerUninstaller(appName, appVersion, installDir, uninstallPath string) {
    keyPath := `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + appName
    k, _, err := registry.CreateKey(registry.CURRENT_USER, keyPath, registry.SET_VALUE|registry.CREATE_SUB_KEY)
    if err != nil {
        return
    }
    defer k.Close()

    k.SetStringValue("DisplayName", appName)
    k.SetStringValue("DisplayVersion", appVersion)
    k.SetStringValue("InstallLocation", installDir)
    k.SetStringValue("UninstallString", `"`+uninstallPath+`" --uninstall`)
    k.SetStringValue("DisplayIcon", filepath.Join(installDir, appName+".exe"))
}