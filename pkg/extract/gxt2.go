package extract

import (
	"encoding/binary"
	"fmt"
	"strings"
)

// GXT2Entry is a decoded RAGE localization string. Hash is the JOAAT key used
// by the game; strings are stored as UTF-8 and may be shared by hash.
type GXT2Entry struct {
	Hash uint32
	Text string
}

// ParseGXT2 reads a GXT2 table without modifying the source bytes.
func ParseGXT2(data []byte) ([]GXT2Entry, error) {
	if len(data) < 16 || string(data[:4]) != "GXT2" {
		return nil, fmt.Errorf("invalid GXT2 header")
	}
	n := binary.LittleEndian.Uint32(data[4:8])
	if uint64(n) > uint64((len(data)-16)/8) {
		return nil, fmt.Errorf("GXT2 entry count %d exceeds file bounds", n)
	}
	end := 8 + int(n)*8
	if end+8 > len(data) || string(data[end:end+4]) != "GXT2" {
		return nil, fmt.Errorf("invalid GXT2 text section")
	}
	textStart := end + 8
	entries := make([]GXT2Entry, 0, n)
	for i := 0; i < int(n); i++ {
		off := 8 + i*8
		h := binary.LittleEndian.Uint32(data[off : off+4])
		rel := binary.LittleEndian.Uint32(data[off+4 : off+8])
		pos := uint64(textStart) + uint64(rel)
		if pos >= uint64(len(data)) {
			return nil, fmt.Errorf("GXT2 string %d offset out of range", i)
		}
		rest := data[pos:]
		z := strings.IndexByte(string(rest), 0)
		if z < 0 {
			return nil, fmt.Errorf("GXT2 string %d is not terminated", i)
		}
		entries = append(entries, GXT2Entry{Hash: h, Text: string(rest[:z])})
	}
	return entries, nil
}

// JOAATHash computes the Jenkins one-at-a-time hash used by RAGE string keys.
func JOAATHash(s string) uint32 {
	var h uint32
	for i := 0; i < len(s); i++ {
		h += uint32(strings.ToLower(s[i : i+1])[0])
		h += h << 10
		h ^= h >> 6
	}
	h += h << 3
	h ^= h >> 11
	h += h << 15
	return h
}
