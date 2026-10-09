package extract

import (
	"bytes"
	"compress/flate"
	"compress/zlib"
	"encoding/binary"
	"fmt"
	"io"
	"math/bits"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// BND4File is a file extracted from a FromSoftware BND4 container.
type BND4File struct {
	ID   int32
	Name string
	Data []byte
}

// ParseBND4 reads common BND4 variants used for message bundles.
func ParseBND4(data []byte) ([]BND4File, error) {
	if len(data) < 64 || string(data[:4]) != "BND4" {
		return nil, fmt.Errorf("invalid BND4 header")
	}
	be := data[8] != 0
	var order binary.ByteOrder = binary.LittleEndian
	if be {
		order = binary.BigEndian
	}
	bitBigEndian := data[10] == 0
	rawFmt := data[49]
	reverse := bitBigEndian || (rawFmt&1 != 0 && rawFmt&0x80 == 0)
	if !reverse {
		rawFmt = bits.Reverse8(rawFmt)
	}
	format := rawFmt
	count := uint64(order.Uint32(data[12:16]))
	headerSize := order.Uint64(data[32:40])
	longOffsets := format&0x10 != 0
	hasCompression := format&0x20 != 0
	hasIDs := format&2 != 0
	hasNames := format&0x0c != 0
	per := uint64(16)
	if longOffsets {
		per += 8
	} else {
		per += 4
	}
	if hasCompression {
		per += 8
	}
	if hasIDs {
		per += 4
	}
	if hasNames {
		per += 4
	}
	if format == 4 {
		per += 8
	}
	if count > 1<<20 || headerSize != per || 64+count*per > uint64(len(data)) {
		return nil, fmt.Errorf("unsupported or invalid BND4 file-header table")
	}
	files := make([]BND4File, 0, count)
	p := uint64(64)
	for n := uint64(0); n < count; n++ {
		flags := data[p]
		if !bitBigEndian {
			flags = bits.Reverse8(flags)
		}
		p += 4
		if int32(order.Uint32(data[p:p+4])) != -1 {
			return nil, fmt.Errorf("invalid BND4 file-header sentinel")
		}
		p += 4
		compressed := order.Uint64(data[p : p+8])
		p += 8
		uncompressed := compressed
		if hasCompression {
			uncompressed = order.Uint64(data[p : p+8])
			p += 8
		}
		var off uint64
		if longOffsets {
			off = order.Uint64(data[p : p+8])
			p += 8
		} else {
			off = uint64(order.Uint32(data[p : p+4]))
			p += 4
		}
		id := int32(-1)
		if hasIDs {
			id = int32(order.Uint32(data[p : p+4]))
			p += 4
		}
		name := ""
		if hasNames {
			no := uint64(order.Uint32(data[p : p+4]))
			p += 4
			if no < uint64(len(data)) {
				name = readBND4Name(data[no:], data[48] != 0, be)
			}
		}
		if format == 4 {
			if p+8 > uint64(len(data)) {
				return nil, fmt.Errorf("truncated BND4 Names1 header")
			}
			id = int32(order.Uint32(data[p : p+4]))
			p += 8
		}
		if compressed > 1<<31 || off > uint64(len(data)) || compressed > uint64(len(data))-off {
			return nil, fmt.Errorf("BND4 file %d data range is invalid (offset=%d compressed=%d length=%d format=%02x)", n, off, compressed, len(data), format)
		}
		payload := append([]byte(nil), data[off:off+compressed]...)
		if flags&1 != 0 && uncompressed > 0 {
			raw, err := inflateBND4(payload, int(uncompressed))
			if err != nil {
				return nil, fmt.Errorf("decompress BND4 file %d: %w", n, err)
			}
			payload = raw
		}
		files = append(files, BND4File{ID: id, Name: name, Data: payload})
	}
	return files, nil
}

func readBND4Name(b []byte, wide, be bool) string {
	if !wide {
		if i := bytes.IndexByte(b, 0); i >= 0 {
			b = b[:i]
		}
		return string(b)
	}
	var u []uint16
	for i := 0; i+1 < len(b); i += 2 {
		v := binary.LittleEndian.Uint16(b[i : i+2])
		if be {
			v = binary.BigEndian.Uint16(b[i : i+2])
		}
		if v == 0 {
			break
		}
		u = append(u, v)
	}
	return string(utf16.Decode(u))
}

func inflateBND4(data []byte, size int) ([]byte, error) {
	if r, e := zlib.NewReader(bytes.NewReader(data)); e == nil {
		defer r.Close()
		out, e := io.ReadAll(io.LimitReader(r, int64(size)+1))
		if e == nil && len(out) == size {
			return out, nil
		}
	}
	r := flate.NewReader(bytes.NewReader(data))
	defer r.Close()
	out, e := io.ReadAll(io.LimitReader(r, int64(size)+1))
	if e != nil {
		return nil, e
	}
	if len(out) != size {
		return nil, fmt.Errorf("size mismatch: got %d, want %d", len(out), size)
	}
	return out, nil
}

func isFMGName(name string) bool { return strings.EqualFold(filepath.Ext(name), ".fmg") }
