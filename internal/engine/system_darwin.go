package engine

import (
	"encoding/binary"
	"fmt"
	"syscall"
)

// totalMemoryFallback reports total system memory in bytes. macOS has no
// /proc/meminfo, so this is the primary path there.
func totalMemoryFallback() (int64, error) {
	raw, err := syscall.Sysctl("hw.memsize")
	if err != nil {
		return 0, err
	}
	// Sysctl returns the raw value as a string with trailing NULs trimmed;
	// restore the fixed 8-byte little-endian layout before decoding.
	buffer := make([]byte, 8)
	if len(raw) > 8 {
		return 0, fmt.Errorf("unexpected hw.memsize length %d", len(raw))
	}
	copy(buffer, raw)
	return int64(binary.LittleEndian.Uint64(buffer)), nil
}
