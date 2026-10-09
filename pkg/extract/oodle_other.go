//go:build !windows

package extract

import "fmt"

func decompressOodle(_ []byte, _ int, _ string) ([]byte, error) {
	return nil, fmt.Errorf("loading the game's Oodle DLL is supported on Windows only")
}
