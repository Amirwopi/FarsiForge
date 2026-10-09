package ffpatch

import (
	"bytes"
	"compress/flate"
	"crypto/md5"
	"crypto/sha256"
	"encoding/binary"
	"hash"
	"io"
	"strings"
	"testing"
)

// ---- minimal Go reader for verification (mirror of the C# reader) ----

type gRecord struct {
	path       string
	src        uint8
	basePath   string
	offset     int64
	size       int64
	payloadOfs int64
	zSize      int64
	rawSize    int64
	md5        []byte
}

type gTarget struct {
	path      string
	mode      uint8
	finalSize int64
	sha256    []byte
	records   []*gRecord
}

type gPatch struct {
	version   uint32
	patchName string
	gameName  string
	gameExe   string
	engine    string
	patchVer  string
	author    string
	desc      string
	createdAt string
	targets   []*gTarget
	blobSize  int64
	blobPos   int64
}

func readStr(r io.Reader) string {
	var n uint32
	binary.Read(r, binary.LittleEndian, &n)
	b := make([]byte, n)
	io.ReadFull(r, b)
	return string(b)
}

func readPatch(data []byte) (*gPatch, error) {
	r := bytes.NewReader(data)
	magic := make([]byte, 4)
	if _, err := io.ReadFull(r, magic); err != nil {
		return nil, err
	}
	if string(magic) != "FFP1" {
		return nil, io.EOF
	}
	p := &gPatch{}
	binary.Read(r, binary.LittleEndian, &p.version)
	if p.version != 1 {
		return nil, io.EOF
	}
	p.patchName = readStr(r)
	p.gameName = readStr(r)
	p.gameExe = readStr(r)
	p.engine = readStr(r)
	p.patchVer = readStr(r)
	p.author = readStr(r)
	p.desc = readStr(r)
	p.createdAt = readStr(r)

	var tcount uint32
	binary.Read(r, binary.LittleEndian, &tcount)
	binary.Read(r, binary.LittleEndian, &p.blobSize)

	for i := uint32(0); i < tcount; i++ {
		t := &gTarget{}
		t.path = readStr(r)
		t.mode = readByte(r)
		binary.Read(r, binary.LittleEndian, &t.finalSize)
		t.sha256 = make([]byte, 32)
		io.ReadFull(r, t.sha256)
		var rcount uint32
		binary.Read(r, binary.LittleEndian, &rcount)
		for j := uint32(0); j < rcount; j++ {
			rec := &gRecord{}
			rec.path = readStr(r)
			rec.src = readByte(r)
			if rec.src == 0 {
				rec.basePath = readStr(r)
				binary.Read(r, binary.LittleEndian, &rec.offset)
				binary.Read(r, binary.LittleEndian, &rec.size)
			} else {
				binary.Read(r, binary.LittleEndian, &rec.payloadOfs)
				binary.Read(r, binary.LittleEndian, &rec.zSize)
				binary.Read(r, binary.LittleEndian, &rec.rawSize)
			}
			rec.md5 = make([]byte, 16)
			io.ReadFull(r, rec.md5)
			t.records = append(t.records, rec)
		}
		p.targets = append(p.targets, t)
	}
	p.blobPos = int64(len(data)) - p.blobSize
	return p, nil
}

func readByte(r io.Reader) uint8 {
	var b [1]byte
	io.ReadFull(r, b[:])
	return b[0]
}

// inflate raw DEFLATE (no zlib header/trailer), matching C# DeflateStream.
func inflateRaw(z []byte, expected int64) ([]byte, error) {
	zr := flate.NewReader(bytes.NewReader(z))
	defer zr.Close()
	out, err := io.ReadAll(zr)
	if err != nil {
		return nil, err
	}
	if int64(len(out)) != expected {
		return nil, io.ErrUnexpectedEOF
	}
	return out, nil
}

// ---- tests ----

func TestWriteAndReadBack(t *testing.T) {
	payloadA := []byte("PATCHED ASSETS CONTENT v2 - longer payload for deflate test 1234567890")
	payloadB := []byte("name=فارسی\nversion=1.0") // UTF-8 Persian

	var buf bytes.Buffer
	w, err := NewWriter(&buf)
	if err != nil {
		t.Fatal(err)
	}
	w.SetMetadata(Metadata{
		PatchName:    "Test Patch",
		GameName:     "TestGame",
		GameExe:      "TestGame.exe",
		Engine:       "Unity",
		PatchVersion: "1.0.0",
		Author:       "tester",
		Description:  "unit test",
	})

	t1, _ := w.AddTarget("data/assets.bin", ModeReplace)
	t1.AddPayload("data/assets.bin", payloadA)

	t2, _ := w.AddTarget("config.txt", ModeReplace)
	t2.AddPayload("config.txt", payloadB)

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	data := buf.Bytes()
	p, err := readPatch(data)
	if err != nil {
		t.Fatalf("readPatch: %v", err)
	}
	if p.patchName != "Test Patch" || p.gameName != "TestGame" || p.gameExe != "TestGame.exe" {
		t.Errorf("metadata mismatch: %+v", p)
	}
	if len(p.targets) != 2 {
		t.Fatalf("want 2 targets, got %d", len(p.targets))
	}
	if p.targets[0].path != "data/assets.bin" || p.targets[0].mode != 1 {
		t.Errorf("target0 wrong: %+v", p.targets[0])
	}
	if p.targets[0].finalSize != int64(len(payloadA)) {
		t.Errorf("finalSize: got %d want %d", p.targets[0].finalSize, len(payloadA))
	}

	// verify sha256 of payloadA matches target sha
	wantSha := sha256.Sum256(payloadA)
	if !bytes.Equal(p.targets[0].sha256, wantSha[:]) {
		t.Errorf("sha256 mismatch target0")
	}

	// verify md5 of record
	wantMd5 := md5.Sum(payloadA)
	if !bytes.Equal(p.targets[0].records[0].md5, wantMd5[:]) {
		t.Errorf("md5 mismatch record0")
	}

	// inflate payload and verify
	blob := data[p.blobPos:]
	rec := p.targets[0].records[0]
	z := blob[rec.payloadOfs : rec.payloadOfs+rec.zSize]
	raw, err := inflateRaw(z, rec.rawSize)
	if err != nil {
		t.Fatalf("inflate: %v", err)
	}
	if !bytes.Equal(raw, payloadA) {
		t.Errorf("inflated payload mismatch")
	}

	// verify Persian payload roundtrips
	rec2 := p.targets[1].records[0]
	z2 := blob[rec2.payloadOfs : rec2.payloadOfs+rec2.zSize]
	raw2, err := inflateRaw(z2, rec2.rawSize)
	if err != nil {
		t.Fatalf("inflate2: %v", err)
	}
	if !bytes.Equal(raw2, payloadB) {
		t.Errorf("persian payload mismatch")
	}
}

func TestCopyFromBaseRequiresFinalHash(t *testing.T) {
	var buf bytes.Buffer
	w, _ := NewWriter(&buf)
	w.SetMetadata(Metadata{PatchName: "x"})
	t1, _ := w.AddTarget("rebuilt.bin", ModeRebuild)
	md5sum := md5.Sum([]byte("base"))
	t1.AddCopyFromBase("rebuilt:copy", "base.bin", 0, 4, md5sum[:])
	// no SetFinalHash -> Close should error
	err := w.Close()
	if err == nil {
		t.Fatal("expected error for copy-from-base without SetFinalHash")
	}
	if !strings.Contains(err.Error(), "SetFinalHash") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCopyFromBaseWithFinalHash(t *testing.T) {
	baseData := []byte("0123456789ABCDEF")
	rebuilt := baseData[:8] // first 8 bytes
	var buf bytes.Buffer
	w, _ := NewWriter(&buf)
	w.SetMetadata(Metadata{PatchName: "x", GameExe: "g.exe"})
	t1, _ := w.AddTarget("rebuilt.bin", ModeRebuild)
	md5sum := md5.Sum(rebuilt)
	t1.AddCopyFromBase("rebuilt:copy", "base.bin", 0, int64(len(rebuilt)), md5sum[:])
	sha := sha256.Sum256(rebuilt)
	t1.SetFinalHash(sha[:], int64(len(rebuilt)))
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	p, err := readPatch(buf.Bytes())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(p.targets) != 1 || len(p.targets[0].records) != 1 {
		t.Fatalf("structure wrong")
	}
	rec := p.targets[0].records[0]
	if rec.src != 0 || rec.basePath != "base.bin" || rec.offset != 0 || rec.size != int64(len(rebuilt)) {
		t.Errorf("copy record wrong: %+v", rec)
	}
	if !bytes.Equal(p.targets[0].sha256, sha[:]) {
		t.Errorf("final sha mismatch")
	}
}

func TestEmptyMetadata(t *testing.T) {
	var buf bytes.Buffer
	w, _ := NewWriter(&buf)
	w.SetMetadata(Metadata{})
	t1, _ := w.AddTarget("a.bin", ModeReplace)
	t1.AddPayload("a", []byte("hello"))
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	p, err := readPatch(buf.Bytes())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if p.createdAt == "" {
		t.Error("CreatedAt should default to now")
	}
}

// verifyHashes replays all payload records and checks md5 + sha consistency.
func verifyHashes(t *testing.T, data []byte) {
	t.Helper()
	p, err := readPatch(data)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	blob := data[p.blobPos:]
	for _, tg := range p.targets {
		var h hash.Hash
		if tg.mode == 1 {
			h = sha256.New()
		}
		for _, rec := range tg.records {
			if rec.src == 1 {
				z := blob[rec.payloadOfs : rec.payloadOfs+rec.zSize]
				raw, err := inflateRaw(z, rec.rawSize)
				if err != nil {
					t.Fatalf("inflate %s: %v", rec.path, err)
				}
				gotMd5 := md5.Sum(raw)
				if !bytes.Equal(gotMd5[:], rec.md5) {
					t.Errorf("md5 mismatch %s", rec.path)
				}
				if h != nil {
					h.Write(raw)
				}
			}
		}
		if h != nil {
			gotSha := h.Sum(nil)
			if !bytes.Equal(gotSha, tg.sha256) {
				t.Errorf("target sha mismatch %s", tg.path)
			}
		}
	}
}

func TestVerifyHashesConsistent(t *testing.T) {
	var buf bytes.Buffer
	w, _ := NewWriter(&buf)
	w.SetMetadata(Metadata{PatchName: "consistency"})
	t1, _ := w.AddTarget("a.bin", ModeReplace)
	t1.AddPayload("a", bytes.Repeat([]byte{0xAB}, 4096))
	t2, _ := w.AddTarget("b.bin", ModeReplace)
	t2.AddPayload("b", []byte("short"))
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	verifyHashes(t, buf.Bytes())
}
