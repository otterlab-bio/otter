package engine

import "syscall"

// totalMemoryFallback reports total system memory in bytes when
// /proc/meminfo is unavailable.
func totalMemoryFallback() (int64, error) {
	var sysInfo syscall.Sysinfo_t
	if err := syscall.Sysinfo(&sysInfo); err != nil {
		return 0, err
	}
	return int64(sysInfo.Totalram) * int64(sysInfo.Unit), nil
}
