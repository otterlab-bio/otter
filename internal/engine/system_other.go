//go:build !linux && !darwin

package engine

import "fmt"

// totalMemoryFallback reports total system memory in bytes. No fallback is
// implemented for this platform; /proc/meminfo is the only source.
func totalMemoryFallback() (int64, error) {
	return 0, fmt.Errorf("no system memory fallback on this platform")
}
