package install

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func zipOf(t *testing.T, names ...string) *zip.Reader {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatalf("creating %s: %v", n, err)
		}
		w.Write([]byte("content of " + n))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	return zr
}

func TestExtractAll(t *testing.T) {
	dest := t.TempDir()
	zr := zipOf(t, "app.exe", "assets/data.bin", "assets/sub/deep.txt")

	var progressed int
	err := ExtractAll(zr, dest, func(name string, i, total int) { progressed++ })
	if err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}
	if progressed != 3 {
		t.Fatalf("progress called %d times, want 3", progressed)
	}
	for _, n := range []string{"app.exe", "assets/data.bin", "assets/sub/deep.txt"} {
		if _, err := os.Stat(filepath.Join(dest, filepath.FromSlash(n))); err != nil {
			t.Errorf("missing extracted file %s: %v", n, err)
		}
	}
}

func TestExtractAllRejectsTraversal(t *testing.T) {
	dest := t.TempDir()
	for _, evil := range []string{"../evil.exe", "..\\..\\evil.exe", "/abs/evil.exe", "C:\\evil.exe"} {
		if err := ExtractAll(zipOf(t, evil), dest, nil); err == nil {
			t.Errorf("traversal path %q was not rejected", evil)
		}
	}
}
