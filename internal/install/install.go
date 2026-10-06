// Package install extracts payload archives to a target directory. It is
// platform-independent; Windows-specific integration (registry, shortcuts)
// lives in the extractor command.
package install

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractAll extracts every entry of zr into destDir, creating directories
// as needed. Entry names are validated to prevent path traversal outside
// destDir. onProgress, if non-nil, is called before each file is written.
func ExtractAll(zr *zip.Reader, destDir string, onProgress func(name string, index, total int)) error {
	total := len(zr.File)
	for i, f := range zr.File {
		target, err := safeJoin(destDir, f.Name)
		if err != nil {
			return err
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("creating directory %s: %w", f.Name, err)
			}
			continue
		}

		if onProgress != nil {
			onProgress(f.Name, i+1, total)
		}

		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("creating parent of %s: %w", f.Name, err)
		}
		if err := extractFile(f, target); err != nil {
			return fmt.Errorf("extracting %s: %w", f.Name, err)
		}
	}
	return nil
}

// safeJoin joins destDir and name, rejecting absolute paths and any path
// that would escape destDir (zip-slip protection). Both slash styles are
// checked because archives built on one OS may be extracted on another.
func safeJoin(destDir, name string) (string, error) {
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") || strings.HasPrefix(name, `\`) ||
		(len(name) >= 2 && name[1] == ':') {
		return "", fmt.Errorf("unsafe absolute path in archive: %q", name)
	}
	parts := strings.FieldsFunc(name, func(r rune) bool { return r == '/' || r == '\\' })
	for _, part := range parts {
		if part == ".." {
			return "", fmt.Errorf("path escapes install directory: %q", name)
		}
	}
	target := filepath.Join(destDir, filepath.FromSlash(name))
	cleanDest := filepath.Clean(destDir)
	if target != cleanDest && !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes install directory: %q", name)
	}
	return target, nil
}

func extractFile(f *zip.File, target string) (err error) {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()

	_, err = io.Copy(out, rc)
	return err
}
