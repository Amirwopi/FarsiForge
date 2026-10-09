package extract

import (
	"crypto/aes"
	"crypto/rsa"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math/big"
	"strings"
)

// ERBHDFile is an entry from an Elden Ring (64-bit) BHD5 index.
type ERBHDFile struct {
	Hash                 uint64
	PaddedSize, Size     uint32
	Offset               int64
	SHAOffset, AESOffset int64
}

// ERBHDIndex contains the file headers needed to address the paired BDT.
type ERBHDIndex struct {
	BigEndian bool
	Files     []ERBHDFile
	Header    []byte
}

const erData0RSAKey = `MIIBCwKCAQEA9Rju2whruXDVQZpfylVEPeNxm7XgMHcDyaaRUIpXQE0qEo+6Y36L
P0xpFvL0H0kKxHwpuISsdgrnMHJ/yj4S61MWzhO8y4BQbw/zJehhDSRCecFJmFBz
3I2JC5FCjoK+82xd9xM5XXdfsdBzRiSghuIHL4qk2WZ/0f/nK5VygeWXn/oLeYBL
jX1S8wSSASza64JXjt0bP/i6mpV2SLZqKRxo7x2bIQrR1yHNekSF2jBhZIgcbtMB
xjCywn+7p954wjcfjxB5VWaZ4hGbKhi1bhYPccht4XnGhcUTWO3NmJWslwccjQ4k
sutLq3uRjLMM0IeTkQO6Pv8/R7UNFtdCWwIERzH8IQ==`

// decryptERBHD5 applies the game's public RSA operation to each 2048-bit block.
// keyPEMBody must be the Base64 body of the archive's PKCS#1 RSA PUBLIC KEY.
func decryptERBHD5(data []byte, keyPEMBody string) ([]byte, error) {
	der, err := base64.StdEncoding.DecodeString(keyPEMBody)
	if err != nil {
		return nil, err
	}
	var key struct {
		N *big.Int
		E int
	}
	if _, err = asn1.Unmarshal(der, &key); err != nil || key.N == nil || key.E == 0 {
		return nil, fmt.Errorf("invalid BHD5 RSA key")
	}
	if len(data)%256 != 0 {
		return nil, fmt.Errorf("RSA-encrypted BHD5 size %d is not a multiple of 256", len(data))
	}
	pub := &rsa.PublicKey{N: key.N, E: key.E}
	out := make([]byte, 0, len(data))
	for i := 0; i < len(data); i += 256 {
		m := new(big.Int).SetBytes(data[i : i+256])
		if m.Cmp(pub.N) >= 0 {
			return nil, fmt.Errorf("BHD5 RSA block %d exceeds modulus", i/256)
		}
		p := new(big.Int).Exp(m, big.NewInt(int64(pub.E)), pub.N).Bytes()
		if len(p) > 256 {
			return nil, fmt.Errorf("invalid decrypted BHD5 block")
		}
		block := make([]byte, 256)
		copy(block[256-len(p):], p)
		out = append(out, block[1:]...)
	}
	return out, nil
}

// ParseERBHD5 parses the plaintext body of an Elden Ring BHD5 header.
func ParseERBHD5(data []byte) (*ERBHDIndex, error) {
	if len(data) < 28 || string(data[:4]) != "BHD5" {
		return nil, fmt.Errorf("invalid BHD5 magic/header")
	}
	be := data[4] == 0
	var order binary.ByteOrder = binary.LittleEndian
	if be {
		order = binary.BigEndian
	}
	fileSize := uint64(order.Uint32(data[12:16]))
	bucketCount := uint64(order.Uint32(data[16:20]))
	bucketsOff := uint64(order.Uint32(data[20:24]))
	if fileSize > uint64(len(data)) || fileSize < 28 || bucketCount > 1<<20 || bucketsOff+bucketCount*8 > fileSize {
		return nil, fmt.Errorf("invalid BHD5 bounds")
	}
	idx := &ERBHDIndex{BigEndian: be, Header: append([]byte(nil), data[:fileSize]...)}
	for i := uint64(0); i < bucketCount; i++ {
		p := bucketsOff + i*8
		n := uint64(order.Uint32(data[p : p+4]))
		off := uint64(order.Uint32(data[p+4 : p+8]))
		if n > 1<<22 || off+n*40 > fileSize {
			return nil, fmt.Errorf("invalid BHD5 bucket %d", i)
		}
		for j := uint64(0); j < n; j++ {
			q := off + j*40
			f := ERBHDFile{Hash: order.Uint64(data[q : q+8]), PaddedSize: order.Uint32(data[q+8 : q+12]), Size: order.Uint32(data[q+12 : q+16]), Offset: int64(order.Uint64(data[q+16 : q+24])), SHAOffset: int64(order.Uint64(data[q+24 : q+32])), AESOffset: int64(order.Uint64(data[q+32 : q+40]))}
			idx.Files = append(idx.Files, f)
		}
	}
	return idx, nil
}

// DecryptERData0BHD decrypts and indexes an Elden Ring Data0.bhd archive.
func DecryptERData0BHD(data []byte) (*ERBHDIndex, error) {
	plain, err := decryptERBHD5(data, erData0RSAKey)
	if err != nil {
		return nil, err
	}
	return ParseERBHD5(plain)
}

// ERPathHash computes FromSoftware's 64-bit ER archive path hash.
func ERPathHash(path string) uint64 {
	p := strings.ToLower(strings.ReplaceAll(path, "\\", "/"))
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	var h uint64
	for _, c := range []byte(p) {
		h = h*0x85 + uint64(c)
	}
	return h
}

// ReadERBHDFile reads one indexed BDT record and applies its declared AES-ECB
// ranges. Hash ranges are retained as metadata but do not affect extraction.
func ReadERBHDFile(index *ERBHDIndex, bdt io.ReaderAt, f ERBHDFile) ([]byte, error) {
	start := f.Offset
	if start < 0 || f.PaddedSize > 1<<30 {
		return nil, fmt.Errorf("BDT entry range is out of bounds")
	}
	data := make([]byte, f.PaddedSize)
	if _, err := bdt.ReadAt(data, start); err != nil {
		return nil, fmt.Errorf("read BDT entry: %w", err)
	}
	if f.AESOffset != 0 {
		o := f.AESOffset
		if o < 0 || o+20 > int64(len(index.Header)) {
			return nil, fmt.Errorf("AES metadata offset is invalid")
		}
		key := index.Header[o : o+16]
		count := int64(index.BigEndianOrder().Uint32(index.Header[o+16 : o+20]))
		if count < 0 || count > 1<<20 || o+20+count*16 > int64(len(index.Header)) {
			return nil, fmt.Errorf("AES range table is invalid")
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		for i := int64(0); i < count; i++ {
			p := o + 20 + i*16
			lo := int64(index.BigEndianOrder().Uint64(index.Header[p : p+8]))
			hi := int64(index.BigEndianOrder().Uint64(index.Header[p+8 : p+16]))
			if lo == -1 || hi == -1 || lo == hi {
				continue
			}
			if lo < 0 || hi < lo || hi > int64(len(data)) || (hi-lo)%aes.BlockSize != 0 {
				return nil, fmt.Errorf("AES range %d is invalid", i)
			}
			for at := lo; at < hi; at += aes.BlockSize {
				block.Decrypt(data[at:at+aes.BlockSize], data[at:at+aes.BlockSize])
			}
		}
	}
	if f.Size > 0 && int64(f.Size) <= int64(len(data)) {
		data = data[:f.Size]
	}
	return data, nil
}

func (i *ERBHDIndex) BigEndianOrder() binary.ByteOrder {
	if i.BigEndian {
		return binary.BigEndian
	}
	return binary.LittleEndian
}
