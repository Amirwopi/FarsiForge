// Package ffpatch implements the FFP1 (FarsiForge Patch v1) binary patch
// format WRITER. It is the byte-exact counterpart of the C# reader in
// patcher/FarsiForgePatcher.cs. See patcher/FORMAT.md for the format spec.
//
// The writer streams a patch to an io.Writer: metadata first, then targets
// and records (with payload offsets resolved against the blob), then the
// concatenated DEFLATE blob at the end.
//
// Usage:
//
//	w, err := ffpatch.NewWriter(out)
//	if err != nil { return err }
//	defer w.Close()
//	w.SetMetadata(ffpatch.Metadata{PatchName: "...", GameName: "...", ...})
//	t, _ := w.AddTarget("path/to/file", ffpatch.ModeReplace)
//	t.AddPayload("logical name", rawData)        // src=1, deflated into blob
//	t.AddCopyFromBase("base.bin", offset, size)  // src=0
//	// ... more targets/records ...
//	if err := w.Close(); err != nil { return err } // writes blob + finalizes
package ffpatch

import (
	"bytes"
	"compress/flate"
	"crypto/md5"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"time"
)

// Mode is the target rebuild mode.
type Mode uint8

const (
	// ModeRebuild rebuilds the target from multiple records (copy-from-base
	// and/or embedded payloads).
	ModeRebuild Mode = 0
	// ModeReplace replaces the target with a single embedded payload record.
	ModeReplace Mode = 1
)

// Metadata is the 8-string patch header metadata.
type Metadata struct {
	PatchName    string
	GameName     string
	GameExe      string
	Engine       string
	PatchVersion string
	Author       string
	Description  string
	CreatedAt    string // ISO 8601; if empty, time.Now().UTC() is used
}

// record is an in-memory record descriptor before blob finalization.
type record struct {
	path        string // logical name
	src         uint8  // 0 = copy-from-base, 1 = payload
	basePath    string
	offset      int64
	size        int64
	payloadData []byte // raw (uncompressed) payload, for src==1
	payloadOfs  int64  // resolved at Close
	zSize       int64
	rawSize     int64
	md5         []byte // 16 bytes
}

// Target is a patch target builder.
type Target struct {
	path      string
	mode      Mode
	finalSize int64
	sha256    []byte // 32 bytes; if nil, computed at Close from records
	records   []*record
}

// Writer is an FFP1 patch writer. Use NewWriter to create one.
type Writer struct {
	w       io.Writer
	meta    Metadata
	targets []*Target
	blob    bytes.Buffer
	closed  bool
	headerW bool
}

// NewWriter creates a new FFP1 writer writing to w. SetMetadata must be
// called before AddTarget. Close finalizes the patch (writes the blob).
func NewWriter(w io.Writer) (*Writer, error) {
	if w == nil {
		return nil, errors.New("ffpatch: nil writer")
	}
	return &Writer{w: w}, nil
}

// SetMetadata sets the patch metadata. Must be called before AddTarget.
func (w *Writer) SetMetadata(m Metadata) {
	w.meta = m
}

// AddTarget adds a target. For ModeReplace, you typically add exactly one
// AddPayload record whose content is the full final file. For ModeRebuild,
// add records in the order they should be streamed to produce the final file.
// If sha256/finalSize are zero, they are computed at Close from the records.
func (w *Writer) AddTarget(path string, mode Mode) (*Target, error) {
	if w.closed {
		return nil, errors.New("ffpatch: writer closed")
	}
	t := &Target{path: path, mode: mode}
	w.targets = append(w.targets, t)
	return t, nil
}

// AddPayload adds an embedded-payload record (src=1). The data is deflated
// into the blob at Close. Returns the record so the caller can set its
// logical name (the first argument).
func (t *Target) AddPayload(logicalName string, data []byte) {
	r := &record{
		path:        logicalName,
		src:         1,
		payloadData: append([]byte(nil), data...),
		rawSize:     int64(len(data)),
	}
	sum := md5.Sum(data)
	r.md5 = sum[:]
	t.records = append(t.records, r)
}

// AddCopyFromBase adds a copy-from-base record (src=0). The md5 of the
// content to be copied must be provided by the caller (it is the md5 of the
// `size` bytes starting at `offset` in `basePath`).
func (t *Target) AddCopyFromBase(logicalName, basePath string, offset, size int64, contentMd5 []byte) {
	r := &record{
		path:     logicalName,
		src:      0,
		basePath: basePath,
		offset:   offset,
		size:     size,
		md5:      append([]byte(nil), contentMd5...),
	}
	t.records = append(t.records, r)
}

// SetFinalHash allows the caller to pre-set the target's final sha256 and
// final size. If not called, they are computed at Close by replaying the
// records (payloads only; copy-from-base content is not known to the writer,
// so for ModeRebuild targets containing copy-from-base records the caller
// MUST call SetFinalHash).
func (t *Target) SetFinalHash(sha []byte, finalSize int64) {
	t.sha256 = append([]byte(nil), sha...)
	t.finalSize = finalSize
}

// Close finalizes the patch: deflates payloads into the blob, resolves
// payload offsets, computes target hashes/sizes where missing, and writes
// the complete FFP1 stream (header + targets + blob) to the underlying writer.
func (w *Writer) Close() error {
	if w.closed {
		return errors.New("ffpatch: already closed")
	}
	w.closed = true

	if w.meta.CreatedAt == "" {
		w.meta.CreatedAt = time.Now().UTC().Format("2006-01-02T15:04:05Z")
	}

	// 1. deflate payloads into blob, resolve offsets, compute missing hashes.
	for _, t := range w.targets {
		// For targets where final hash/size not pre-set, compute from records.
		// We can only compute when ALL records are payloads (src==1). For
		// copy-from-base records the writer doesn't have the bytes, so the
		// caller must have called SetFinalHash.
		needHash := len(t.sha256) == 0
		needSize := t.finalSize == 0
		var recomputed hash.Hash
		var recomputedSize int64
		if needHash || needSize {
			recomputed = sha256.New()
		}
		for _, r := range t.records {
			if r.src == 1 {
				// deflate raw payload into blob
				var zbuf bytes.Buffer
				zw, err := flate.NewWriter(&zbuf, flate.DefaultCompression)
				if err != nil {
					return fmt.Errorf("ffpatch: flate writer: %w", err)
				}
				if _, err := zw.Write(r.payloadData); err != nil {
					return fmt.Errorf("ffpatch: flate write: %w", err)
				}
				if err := zw.Close(); err != nil {
					return fmt.Errorf("ffpatch: flate close: %w", err)
				}
				r.payloadOfs = int64(w.blob.Len())
				r.zSize = int64(zbuf.Len())
				if needHash || needSize {
					recomputed.Write(r.payloadData)
					recomputedSize += int64(len(r.payloadData))
				}
				// append to blob
				w.blob.Write(zbuf.Bytes())
			} else {
				if needHash || needSize {
					return fmt.Errorf("ffpatch: target %q has copy-from-base record but no pre-set final hash/size; call SetFinalHash", t.path)
				}
			}
		}
		if needSize {
			t.finalSize = recomputedSize
		}
		if needHash {
			t.sha256 = recomputed.Sum(nil)
		}
	}

	// 2. write header.
	var hdr bytes.Buffer
	hdr.WriteString("FFP1")
	binary.Write(&hdr, binary.LittleEndian, uint32(1)) // version

	writeStr(&hdr, w.meta.PatchName)
	writeStr(&hdr, w.meta.GameName)
	writeStr(&hdr, w.meta.GameExe)
	writeStr(&hdr, w.meta.Engine)
	writeStr(&hdr, w.meta.PatchVersion)
	writeStr(&hdr, w.meta.Author)
	writeStr(&hdr, w.meta.Description)
	writeStr(&hdr, w.meta.CreatedAt)

	binary.Write(&hdr, binary.LittleEndian, uint32(len(w.targets)))
	binary.Write(&hdr, binary.LittleEndian, int64(w.blob.Len()))

	// 3. write targets + records.
	for _, t := range w.targets {
		writeStr(&hdr, t.path)
		hdr.WriteByte(byte(t.mode))
		binary.Write(&hdr, binary.LittleEndian, t.finalSize)
		hdr.Write(t.sha256)
		binary.Write(&hdr, binary.LittleEndian, uint32(len(t.records)))
		for _, r := range t.records {
			writeStr(&hdr, r.path)
			hdr.WriteByte(r.src)
			if r.src == 0 {
				writeStr(&hdr, r.basePath)
				binary.Write(&hdr, binary.LittleEndian, r.offset)
				binary.Write(&hdr, binary.LittleEndian, r.size)
			} else {
				binary.Write(&hdr, binary.LittleEndian, r.payloadOfs)
				binary.Write(&hdr, binary.LittleEndian, r.zSize)
				binary.Write(&hdr, binary.LittleEndian, r.rawSize)
			}
			hdr.Write(r.md5)
		}
	}

	// 4. write header then blob.
	if _, err := w.w.Write(hdr.Bytes()); err != nil {
		return fmt.Errorf("ffpatch: write header: %w", err)
	}
	if _, err := w.w.Write(w.blob.Bytes()); err != nil {
		return fmt.Errorf("ffpatch: write blob: %w", err)
	}
	return nil
}

func writeStr(w io.Writer, s string) {
	b := []byte(s)
	binary.Write(w, binary.LittleEndian, uint32(len(b)))
	w.Write(b)
}
