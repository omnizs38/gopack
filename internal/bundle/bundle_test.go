package bundle

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

// fakeBinary simulates an extractor template: arbitrary bytes followed by a
// payload written by Write.
func fakeBinary(t *testing.T, files map[string]string) (*bytes.Reader, int64) {
	t.Helper()

	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("creating zip entry: %v", err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("writing zip entry: %v", err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("closing zip: %v", err)
	}

	var bin bytes.Buffer
	bin.WriteString("MZ-fake-exe-template-bytes")
	if err := Write(&bin, zipBuf.Bytes()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	return bytes.NewReader(bin.Bytes()), int64(bin.Len())
}

func TestWriteReadRoundTrip(t *testing.T) {
	want := map[string]string{
		"myapp.exe":        "fake-exe-content",
		"assets/readme.md": "hello from the payload",
		"__metadata.json":  `{"appName":"MyApp"}`,
	}
	r, size := fakeBinary(t, want)

	zr, err := Read(r, size)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(zr.File) != len(want) {
		t.Fatalf("got %d files, want %d", len(zr.File), len(want))
	}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("opening %s: %v", f.Name, err)
		}
		got, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatalf("reading %s: %v", f.Name, err)
		}
		if string(got) != want[f.Name] {
			t.Errorf("file %s: got %q, want %q", f.Name, got, want[f.Name])
		}
	}
}

func TestReadNoPayload(t *testing.T) {
	plain := bytes.NewReader([]byte("just some binary without a payload"))
	if _, err := Read(plain, int64(plain.Len())); err != ErrNoPayload {
		t.Fatalf("got %v, want ErrNoPayload", err)
	}
}

func TestReadTooSmall(t *testing.T) {
	tiny := bytes.NewReader([]byte("tiny"))
	if _, err := Read(tiny, 4); err != ErrNoPayload {
		t.Fatalf("got %v, want ErrNoPayload", err)
	}
}

func TestReadCorruptSize(t *testing.T) {
	r, size := fakeBinary(t, map[string]string{"a.txt": "a"})
	raw, _ := io.ReadAll(r)
	// Inflate the payload size in the footer so it points before the binary start.
	raw[size-14] = 0xFF
	raw[size-13] = 0xFF
	raw[size-12] = 0xFF
	raw[size-11] = 0xFF
	if _, err := Read(bytes.NewReader(raw), size); err == nil {
		t.Fatal("expected error for corrupt footer, got nil")
	}
}
