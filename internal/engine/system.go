package engine

import (
	"bufio"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/otterlab-bio/otter/internal/logger"
)

// SystemInfo stores system resource information
type SystemInfo struct {
	TotalCores   int   // total CPU cores
	AvailableMem int64 // available memory (MB)
	TotalMem     int64 // total memory (MB)
}

// GetSystemInfo returns system resource information
func GetSystemInfo() (*SystemInfo, error) {
	info := &SystemInfo{}

	// Get CPU core count
	info.TotalCores = runtime.NumCPU()
	if info.TotalCores <= 0 {
		info.TotalCores = 1
		logger.Warn("Failed to detect CPU cores, using default: 1")
	}

	// Get memory info from /proc/meminfo
	totalMemBytes, err := readTotalMemory()
	if err != nil {
		logger.Warnf("Failed to read memory from /proc/meminfo: %v", err)
		// Use syscall fallback
		var sysInfo syscall.Sysinfo_t
		if err2 := syscall.Sysinfo(&sysInfo); err2 != nil {
			return nil, fmt.Errorf("failed to get system memory info: %w", err)
		}
		// Assume 4096-byte pages (most Linux systems)
		totalMemBytes = int64(sysInfo.Totalram) * 4096
	}

	info.TotalMem = totalMemBytes / (1024 * 1024) // convert to MB

	// Reserve 20% of total memory for system overhead
	info.AvailableMem = int64(float64(info.TotalMem) * 0.8)

	logger.Debugf("System info: %d cores, %d MB total memory, %d MB available (80%%)",
		info.TotalCores, info.TotalMem, info.AvailableMem)

	return info, nil
}

// readTotalMemory reads total system memory from /proc/meminfo
func readTotalMemory() (int64, error) {
	meminfoFile, err := open("/proc/meminfo")
	if err != nil {
		return 0, err
	}
	defer meminfoFile.Close()

	scanner := bufio.NewScanner(meminfoFile)
	for scanner.Scan() {
		line := scanner.Text()
		// Look for "MemTotal:" line
		if strings.HasPrefix(line, "MemTotal:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				// Parse the value (unit: kB)
				memKB, err := strconv.ParseInt(parts[1], 10, 64)
				if err != nil {
					return 0, fmt.Errorf("failed to parse memory value: %w", err)
				}
				// Convert to bytes
				return memKB * 1024, nil
			}
		}
	}

	return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
}

// open is a wrapper function that can be mocked in tests
var open = func(path string) (meminfoFile, error) {
	f, err := os.Open(path)
	return meminfoFile{f}, err
}

// meminfoFile is a wrapper around os.File for test mocking
type meminfoFile struct {
	*os.File
}

// Read implements io.Reader for bufio.Scanner compatibility.
func (f meminfoFile) Read(p []byte) (n int, err error) {
	return f.File.Read(p)
}

// ParseMemory parses a memory string into MB.
// Supported formats: "100G", "2000M", "10G", "512M", "8GB", "1000MB", "1T", "2TB"
func ParseMemory(memStr string) (int64, error) {
	if memStr == "" {
		return 0, nil
	}

	// Trim whitespace and convert to uppercase
	memStr = strings.TrimSpace(memStr)
	memStr = strings.ToUpper(memStr)

	// Extract numeric and unit portions
	var numStr, unitStr string
	for i, char := range memStr {
		if (char >= '0' && char <= '9') || char == '.' {
			numStr += string(char)
		} else {
			unitStr = memStr[i:]
			break
		}
	}

	if numStr == "" {
		return 0, fmt.Errorf("invalid memory format: %s", memStr)
	}

	// Parse the numeric value
	value, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid memory value: %s", numStr)
	}

	// Apply unit multiplier
	unitMultiplier := int64(1)
	switch unitStr {
	case "", "B", "BYTE", "BYTES":
		unitMultiplier = 1
	case "K", "KB", "KILOBYTE", "KILOBYTES":
		unitMultiplier = 1024
	case "M", "MB", "MEGABYTE", "MEGABYTES":
		unitMultiplier = 1024 * 1024
	case "G", "GB", "GIB", "GIBYTE", "GIBYTES", "GIGABYTE", "GIGABYTES":
		unitMultiplier = 1024 * 1024 * 1024
	case "T", "TB", "TIB", "TIBYTE", "TIBYTES", "TERABYTE", "TERABYTES":
		unitMultiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unknown memory unit: %s", unitStr)
	}

	// Convert to bytes, then to MB
	totalBytes := int64(value * float64(unitMultiplier))
	mb := totalBytes / (1024 * 1024)

	return mb, nil
}

func formatSlurmMemory(memory string) (string, error) {
	memoryMB, err := ParseMemory(memory)
	if err != nil {
		return "", fmt.Errorf("parse Slurm memory %q: %w", memory, err)
	}
	if memoryMB <= 0 {
		return "", fmt.Errorf("Slurm memory must be positive, got %q", memory)
	}
	return fmt.Sprintf("%dM", memoryMB), nil
}

// ValidateLocalResources validates that local resources meet requirements
func ValidateLocalResources(requestedCores int, requestedMemory string) error {
	// Get system resource info
	sysInfo, err := GetSystemInfo()
	if err != nil {
		return fmt.Errorf("failed to get system info: %w", err)
	}

	// Validate CPU cores
	if requestedCores <= 0 {
		return fmt.Errorf("invalid requested cores: %d", requestedCores)
	}

	if requestedCores > sysInfo.TotalCores {
		return fmt.Errorf("requested %d cores exceeds system available %d cores",
			requestedCores, sysInfo.TotalCores)
	}

	// Validate memory
	if requestedMemory != "" {
		requestedMemMB, err := ParseMemory(requestedMemory)
		if err != nil {
			return fmt.Errorf("invalid memory format: %w", err)
		}

		if requestedMemMB <= 0 {
			return fmt.Errorf("invalid requested memory: %s", requestedMemory)
		}

		if requestedMemMB > sysInfo.AvailableMem {
			return fmt.Errorf("requested %dMB exceeds available %dMB (80%% of %dMB total)",
				requestedMemMB, sysInfo.AvailableMem, sysInfo.TotalMem)
		}
	}

	// Warn if requested cores approach system total
	if requestedCores >= sysInfo.TotalCores {
		logger.Warnf("Requested %d cores equals total cores (%d). System may become unresponsive.",
			requestedCores, sysInfo.TotalCores)
	}

	logger.Infof("Resource validation passed: %d/%d cores, %s/%dMB memory available",
		requestedCores, sysInfo.TotalCores, requestedMemory, sysInfo.AvailableMem)

	return nil
}

// ValidateParallelJobs validates that the parallel job count is reasonable
func ValidateParallelJobs(parallelJobs int) error {
	sysInfo, err := GetSystemInfo()
	if err != nil {
		return fmt.Errorf("failed to get system info: %w", err)
	}

	if parallelJobs <= 0 {
		return fmt.Errorf("invalid parallel jobs: %d", parallelJobs)
	}

	// Recommend: parallel jobs should not exceed CPU cores
	if parallelJobs > sysInfo.TotalCores {
		return fmt.Errorf("parallel jobs %d exceeds CPU cores %d",
			parallelJobs, sysInfo.TotalCores)
	}

	// Recommend: parallel jobs should not exceed 75% of total cores
	maxRecommended := int(float64(sysInfo.TotalCores) * 0.75)
	if parallelJobs > maxRecommended {
		logger.Warnf("High parallelism (%d jobs on %d cores) may impact performance. Recommended: <= %d",
			parallelJobs, sysInfo.TotalCores, maxRecommended)
	} else {
		logger.Debugf("Parallel jobs validation passed: %d/%d cores", parallelJobs, sysInfo.TotalCores)
	}

	return nil
}
