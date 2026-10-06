// Command packer builds a Windows installer by fusing an extractor template
// with the application payload.
//
// Configuration can be supplied as a JSON file and/or CLI flags; explicitly
// set flags override the JSON values.
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/omnizs38/gopack/internal/bundle"
	"github.com/omnizs38/gopack/internal/metadata"
)

// config mirrors the JSON config file format.
type config struct {
	Src       string `json:"src"`
	Out       string `json:"out"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Exe       string `json:"exe"`
	Publisher string `json:"publisher"`
	Template  string `json:"template"`
}

func main() {
	log.SetPrefix("gopack: ")

	configPath := flag.String("config", "", "Path to a JSON config file")
	src := flag.String("src", "", "Source application directory")
	out := flag.String("out", "", "Output setup file name")
	name := flag.String("name", "", "Application name")
	version := flag.String("version", "", "Application version")
	mainExe := flag.String("exe", "", "Main executable name (relative to src)")
	publisher := flag.String("publisher", "", "Publisher name shown in Windows settings")
	template := flag.String("template", "", "Extractor template binary")
	flag.Parse()

	cfg := config{
		Src:      "./myapp",
		Out:      "setup.exe",
		Template: "extractor_template.exe",
	}
	if *configPath != "" {
		data, err := os.ReadFile(*configPath)
		if err != nil {
			log.Fatalf("reading config: %v", err)
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			log.Fatalf("parsing config %s: %v", *configPath, err)
		}
	}
	// Explicitly provided flags override the config file / defaults.
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "src":
			cfg.Src = *src
		case "out":
			cfg.Out = *out
		case "name":
			cfg.Name = *name
		case "version":
			cfg.Version = *version
		case "exe":
			cfg.Exe = *mainExe
		case "publisher":
			cfg.Publisher = *publisher
		case "template":
			cfg.Template = *template
		}
	})

	if err := run(cfg); err != nil {
		log.Fatal(err)
	}
}

func run(cfg config) error {
	meta := metadata.Metadata{
		AppName:    cfg.Name,
		AppVersion: cfg.Version,
		MainExe:    cfg.Exe,
		Publisher:  cfg.Publisher,
	}
	if err := meta.Validate(); err != nil {
		return fmt.Errorf("invalid metadata: %w", err)
	}

	info, err := os.Stat(cfg.Src)
	if err != nil {
		return fmt.Errorf("source directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("source %s is not a directory", cfg.Src)
	}
	if _, err := os.Stat(filepath.Join(cfg.Src, filepath.FromSlash(cfg.Exe))); err != nil {
		return fmt.Errorf("main executable %q not found in %s", cfg.Exe, cfg.Src)
	}

	zipData, fileCount, err := buildPayload(cfg.Src, meta)
	if err != nil {
		return err
	}

	tpl, err := os.Open(cfg.Template)
	if err != nil {
		return fmt.Errorf("opening extractor template (build it with `make extractor`): %w", err)
	}
	defer tpl.Close()

	outFile, err := os.Create(cfg.Out)
	if err != nil {
		return fmt.Errorf("creating output: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, tpl); err != nil {
		return fmt.Errorf("writing template: %w", err)
	}
	if err := bundle.Write(outFile, zipData); err != nil {
		return err
	}
	if err := outFile.Close(); err != nil {
		return err
	}

	stat, _ := os.Stat(cfg.Out)
	fmt.Printf("%s built: %s v%s, %d files, %.1f MB total\n",
		cfg.Out, meta.AppName, meta.AppVersion, fileCount, float64(stat.Size())/1024/1024)
	return nil
}

// buildPayload zips srcDir and embeds the metadata file.
func buildPayload(srcDir string, meta metadata.Metadata) ([]byte, int, error) {
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)

	fileCount := 0
	walkErr := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		// Zip entries always use forward slashes, even when packed on Windows.
		w, err := zipWriter.Create(filepath.ToSlash(relPath))
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(w, f)
		closeErr := f.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
		fileCount++
		return nil
	})
	if walkErr != nil {
		return nil, 0, fmt.Errorf("walking source directory: %w", walkErr)
	}

	metaJSON, err := meta.Encode()
	if err != nil {
		return nil, 0, err
	}
	w, err := zipWriter.Create(metadata.MetadataFile)
	if err != nil {
		return nil, 0, err
	}
	if _, err := w.Write(metaJSON); err != nil {
		return nil, 0, err
	}
	if err := zipWriter.Close(); err != nil {
		return nil, 0, fmt.Errorf("writing payload archive: %w", err)
	}
	return buf.Bytes(), fileCount, nil
}
