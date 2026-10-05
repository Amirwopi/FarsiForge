// Package hash provides SHA256 file hashing for integrity verification.
package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// File computes the SHA256 hash of a file.
// Uses streaming reads to handle large files without loading them into memory.
func File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for hashing: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// String computes the SHA256 hash of a string.
func String(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// Bytes computes the SHA256 hash of a byte slice.
func Bytes(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// Verify checks if a file matches an expected hash.
func Verify(path, expectedHash string) (bool, error) {
	actual, err := File(path)
	if err != nil {
		return false, err
	}
	return actual == expectedHash, nil
}
