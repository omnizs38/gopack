package main

import (
    "archive/zip"
    "bytes"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log"
    "os"
    "path/filepath"

    "gopack/internal/metadata"
)

func main() {
    appDir := flag.String("src", "./myapp", "Source application directory")
    outFile := flag.String("out", "setup.exe", "Output setup file name")
    appName := flag.String("name", "MyApp", "Application name")
    appVersion := flag.String("version", "1.0.0", "Application version")
    mainExe := flag.String("exe", "myapp.exe", "Main executable name")
    extractorTemplate := flag.String("template", "extractor_template.exe", "Extractor template binary")
    
    flag.Parse()

    buf := new(bytes.Buffer)
    zipWriter := zip.NewWriter(buf)

    err := filepath.WalkDir(*appDir, func(path string, d os.DirEntry, err error) error {
        if err != nil || d.IsDir() {
            return err
        }
        relPath, _ := filepath.Rel(*appDir, path)

        w, err := zipWriter.Create(relPath)
        if err != nil {
            return err
        }
        f, err := os.Open(path)
        if err != nil {
            return err
        }
        defer f.Close()
        _, err = io.Copy(w, f)
        return err
    })
    if err != nil {
        log.Fatalf("Failed to walk directory: %v", err)
    }

    meta := metadata.Metadata{AppName: *appName, AppVersion: *appVersion, MainExe: *mainExe}
    metaJSON, _ := json.Marshal(meta)
    w, err := zipWriter.Create("__metadata.json")
    if err != nil {
        log.Fatal(err)
    }
    w.Write(metaJSON)
    zipWriter.Close()

    exeFile, err := os.Open(*extractorTemplate)
    if err != nil {
        log.Fatalf("Template not found: %v", err)
    }
    defer exeFile.Close()

    out, err := os.Create(*outFile)
    if err != nil {
        log.Fatal(err)
    }
    defer out.Close()

    io.Copy(out, exeFile)
    buf.WriteTo(out)

    fmt.Printf("%s built successfully.\n", *outFile)
}