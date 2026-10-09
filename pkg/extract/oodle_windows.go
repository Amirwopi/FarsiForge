//go:build windows

package extract

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

func decompressOodle(src []byte, outSize int, dllPath string) ([]byte, error) {
	if len(src) == 0 || outSize <= 0 {
		return nil, fmt.Errorf("invalid Oodle buffer sizes")
	}
	dll, err := syscall.LoadDLL(dllPath)
	if err != nil {
		return nil, fmt.Errorf("load oo2core DLL: %w", err)
	}
	defer dll.Release()
	proc, err := dll.FindProc("OodleLZ_Decompress")
	if err != nil {
		return nil, fmt.Errorf("find OodleLZ_Decompress: %w", err)
	}
	dst := make([]byte, outSize)
	decoded, _, callErr := proc.Call(uintptr(unsafe.Pointer(&src[0])), uintptr(len(src)), uintptr(unsafe.Pointer(&dst[0])), uintptr(len(dst)), 0, 0, 0, 0, 0, 0, 0, 0, 0, 3)
	runtime.KeepAlive(src)
	runtime.KeepAlive(dst)
	if decoded == 0 {
		return nil, fmt.Errorf("Oodle decompression failed: %v", callErr)
	}
	if decoded > uintptr(len(dst)) {
		return nil, fmt.Errorf("Oodle returned impossible output size %d", decoded)
	}
	return dst[:decoded], nil
}
