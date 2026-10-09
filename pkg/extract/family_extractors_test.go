package extract

import (
	"encoding/binary"
	"testing"
)

func TestParseGXT2(t *testing.T) {
	b := make([]byte, 8+8+8+6)
	copy(b, "GXT2")
	binary.LittleEndian.PutUint32(b[4:], 1)
	binary.LittleEndian.PutUint32(b[8:], 0x12345678)
	binary.LittleEndian.PutUint32(b[12:], 0)
	copy(b[16:], "GXT2")
	binary.LittleEndian.PutUint32(b[20:], 6)
	copy(b[24:], "Hello\x00")
	got, err := ParseGXT2(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Text != "Hello" || got[0].Hash != 0x12345678 {
		t.Fatalf("unexpected result: %#v", got)
	}
}

func TestParseFMGV2Wide(t *testing.T) {
	// Header (40), one group (16), one absolute string offset (8), then text.
	b := make([]byte, 70)
	b[0] = 0
	b[1] = 0
	b[2] = 2
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)))
	b[8] = 1
	binary.LittleEndian.PutUint32(b[12:], 1)
	binary.LittleEndian.PutUint32(b[16:], 1)
	b[20] = 0xff
	binary.LittleEndian.PutUint64(b[24:], 56)
	binary.LittleEndian.PutUint32(b[40:], 0)
	binary.LittleEndian.PutUint32(b[44:], 42)
	binary.LittleEndian.PutUint32(b[48:], 42)
	binary.LittleEndian.PutUint64(b[56:], 64)
	copy(b[64:], []byte{'H', 0, 'i', 0, 0, 0})
	got, err := ParseFMG(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != 42 || got[0].Text != "Hi" {
		t.Fatalf("unexpected result: %#v", got)
	}
}
