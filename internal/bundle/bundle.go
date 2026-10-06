// Package bundle implements the gopack payload format. A built installer is:
//
//	[extractor executable][LZ4(zip payload)][8-byte LE payload size]["GPKLZ4"]
//
// The packer appends the payload with Write, and the extractor reads it back
// from its own binary with Read. Keeping both sides in one package guarantees
// the two binaries can never drift out of sync again.
package bundle

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/pierrec/lz4/v4"
)

// Magic marks the end of a gopack payload footer.
const Magic = "GPKLZ4"

// footerSize is the 8-byte payload size plus the 6-byte magic marker.
const footerSize = int64(8 + len(Magic))

// ErrNoPayload is returned when a binary has no gopack payload appended.
var ErrNoPayload = errors.New("gopack payload not found (GPKLZ4 magic missing)")

// Write compresses zipData with LZ4 and appends it, plus the footer, to w.
// Typically the extractor template has already been copied to w.
func Write(w io.Writer, zipData []byte) error {
	var compressed bytes.Buffer
	zw := lz4.NewWriter(&compressed)
	if _, err := zw.Write(zipData); err != nil {
		return fmt.Errorf("compressing payload: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("finalizing compression: %w", err)
	}

	if _, err := w.Write(compressed.Bytes()); err != nil {
		return fmt.Errorf("writing payload: %w", err)
	}

	footer := make([]byte, footerSize)
	binary.LittleEndian.PutUint64(footer[:8], uint64(compressed.Len()))
	copy(footer[8:], Magic)
	if _, err := w.Write(footer); err != nil {
		return fmt.Errorf("writing footer: %w", err)
	}
	return nil
}

// Read locates and decompresses the payload appended to a binary of the
// given total size, and returns it as a zip.Reader.
func Read(r io.ReaderAt, size int64) (*zip.Reader, error) {
	if size < footerSize {
		return nil, ErrNoPayload
	}

	footer := make([]byte, footerSize)
	if _, err := r.ReadAt(footer, size-footerSize); err != nil {
		return nil, fmt.Errorf("reading footer: %w", err)
	}
	if string(footer[8:]) != Magic {
		return nil, ErrNoPayload
	}

	lz4Size := int64(binary.LittleEndian.Uint64(footer[:8]))
	lz4Start := size - footerSize - lz4Size
	if lz4Size <= 0 || lz4Start < 0 {
		return nil, fmt.Errorf("corrupt payload footer: invalid size %d", lz4Size)
	}

	lz4Data := make([]byte, lz4Size)
	if _, err := r.ReadAt(lz4Data, lz4Start); err != nil {
		return nil, fmt.Errorf("reading payload: %w", err)
	}

	var zipBuf bytes.Buffer
	if _, err := io.Copy(&zipBuf, lz4.NewReader(bytes.NewReader(lz4Data))); err != nil {
		return nil, fmt.Errorf("decompressing payload: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(zipBuf.Bytes()), int64(zipBuf.Len()))
	if err != nil {
		return nil, fmt.Errorf("opening embedded archive: %w", err)
	}
	return zr, nil
}
