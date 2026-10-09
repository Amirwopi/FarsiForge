package extract

import (
	"encoding/binary"
	"fmt"
	"unicode/utf16"
)

// FMGEntry contains a FromSoftware message ID and its decoded text.
type FMGEntry struct {
	ID   int32
	Text string
}

// ParseFMG decodes FMG v1/v2 tables. V2 uses 32-bit IDs and variable-width
// offsets; the byte-order and Unicode flags are read from the header.
func ParseFMG(data []byte) ([]FMGEntry, error) {
	if len(data) < 24 {
		return nil, fmt.Errorf("FMG header is truncated")
	}
	if data[0] != 0 || data[3] != 0 || data[8] != 1 {
		return nil, fmt.Errorf("invalid FMG signature/flags")
	}
	be := data[1] != 0
	var order binary.ByteOrder = binary.LittleEndian
	if be {
		order = binary.BigEndian
	}
	version := data[2]
	fileSize := int64(order.Uint32(data[4:8]))
	unicodeText := true
	if fileSize < 24 || fileSize > int64(len(data)) {
		return nil, fmt.Errorf("invalid FMG file size %d", fileSize)
	}
	groupCount := int(order.Uint32(data[12:16]))
	stringCount := int(order.Uint32(data[16:20]))
	if groupCount < 0 || groupCount > 1<<20 || stringCount < 0 || stringCount > 1<<22 {
		return nil, fmt.Errorf("invalid FMG counts")
	}
	wide := version == 2
	ptrSize, groupStart, groupSize := 4, 28, 12
	offAt := 20
	if wide {
		ptrSize, groupStart, groupSize, offAt = 8, 40, 16, 24
	}
	if groupStart+groupCount*groupSize > int(fileSize) {
		return nil, fmt.Errorf("FMG groups exceed file bounds")
	}
	var offsets int64
	if ptrSize == 8 {
		offsets = int64(order.Uint64(data[offAt : offAt+8]))
	} else {
		offsets = int64(order.Uint32(data[offAt : offAt+4]))
	}
	if offsets < 0 || offsets+int64(stringCount)*int64(ptrSize) > fileSize {
		return nil, fmt.Errorf("FMG string-offset table out of range")
	}
	entries := make([]FMGEntry, 0, stringCount)
	for g := 0; g < groupCount; g++ {
		p := groupStart + g*groupSize
		first, last := int32(order.Uint32(data[p+4:p+8])), int32(order.Uint32(data[p+8:p+12]))
		if last < first || int64(last)-int64(first) > 1<<22 {
			continue
		}
		for j := int64(0); j <= int64(last)-int64(first); j++ {
			id := int32(int64(first) + j)
			idx := int64(order.Uint32(data[p:p+4])) + j
			op := offsets + idx*int64(ptrSize)
			if op < 0 || op+int64(ptrSize) > fileSize {
				continue
			}
			var rel uint64
			if ptrSize == 8 {
				rel = order.Uint64(data[op : op+8])
			} else {
				rel = uint64(order.Uint32(data[op : op+4]))
			}
			if rel == 0 {
				continue
			}
			if rel > uint64(fileSize) {
				continue
			}
			start := int64(rel)
			if start < 0 || start >= fileSize {
				continue
			}
			text, ok := readFMGString(data[start:fileSize], unicodeText, be)
			if ok && text != "" {
				entries = append(entries, FMGEntry{ID: id, Text: text})
			}
		}
	}
	return entries, nil
}

func readFMGString(b []byte, wide, be bool) (string, bool) {
	if !wide {
		for i, c := range b {
			if c == 0 {
				return string(b[:i]), true
			}
		}
		return "", false
	}
	var u []uint16
	for i := 0; i+1 < len(b); i += 2 {
		v := binary.LittleEndian.Uint16(b[i : i+2])
		if be {
			v = binary.BigEndian.Uint16(b[i : i+2])
		}
		if v == 0 {
			return string(utf16.Decode(u)), true
		}
		u = append(u, v)
	}
	return "", false
}
