# SLURM Resource Validation Test Results

Date: 2026-01-12
Test Environment: login2 cluster (SLURM 21.08.8-2)
Go Version: go1.25.5 (node conda environment)

## Cluster Information

| Partition | Nodes | Node Specs | State |
|-----------|-------|------------|-------|
| cpu48c | 1 (comput1) | 48 cores, 763GB RAM | idle |
| cpu52c | 1 (comput6) | 52 cores | mix |
| cpu112c | 3 (comput2-4) | 112 cores, 257GB RAM | mix/alloc |
| fat | 1 (comput5) | 96 cores, 3TB RAM | mix |
| gpu | 1 (gpu1) | GPU | drain |

## Bugs Found and Fixed

### Bug 1: `getPartitionNodes()` - Incorrect field parsing
**File**: `internal/engine/slurm.go:96`

**Issue**: Used `strings.Fields(line)` which splits on whitespace, but sinfo output is comma-separated:
```
comput1,idle,0/48/0/48,763000
```

**Fix**: Changed to `strings.Split(line, ",")` to properly parse comma-separated values.

**Impact**: Critical - All node resource validations were failing silently.

---

### Bug 2: `ValidateSlurmPartition()` - Missing empty output check
**File**: `internal/engine/slurm.go:28-34`

**Issue**: Function only checked command exit code, but `sinfo -h -p <partition>` returns exit code 0 even for non-existent partitions (with empty output).

**Fix**: Added check for empty output:
```go
if len(strings.TrimSpace(string(output))) == 0 {
    return fmt.Errorf("partition '%s' does not exist", partition)
}
```

**Impact**: High - Invalid partitions were accepted as valid.

---

### Bug 3: Missing Terabyte (TB) memory unit support
**File**: `internal/engine/system.go:136-149`

**Issue**: `ParseMemory()` didn't support "T" or "TB" units, causing validation to fail for large memory requests.

**Fix**: Added case for TB units:
```go
case "T", "TB", "TERABYTE", "TERABYTES":
    unitMultiplier = 1024 * 1024 * 1024 * 1024
```

**Impact**: Medium - Prevented submitting jobs with TB-scale memory requests.

---

### Bug 4: Missing empty partition name validation
**File**: `internal/engine/slurm.go:28-31`

**Issue**: Empty partition string was not validated before calling sinfo.

**Fix**: Added validation:
```go
if partition == "" {
    return fmt.Errorf("partition name cannot be empty")
}
```

**Impact**: Low - Edge case, but improves error messages.

---

### Enhancement: Add zero cores validation
**File**: `internal/engine/slurm.go:47-50`

**Issue**: Zero or negative core counts were not validated.

**Fix**: Added validation:
```go
if requestedCores <= 0 {
    return fmt.Errorf("requested cores must be greater than 0, got %d", requestedCores)
}
```

**Impact**: Low - Improves error detection for invalid configurations.

---

## Test Results

### Test 1: Memory Parsing ✅
| Input | Expected (MB) | Actual (MB) | Status |
|-------|---------------|-------------|--------|
| 100G | 102400 | 102400 | ✅ PASS |
| 2000M | 2000 | 2000 | ✅ PASS |
| 8G | 8192 | 8192 | ✅ PASS |
| 512M | 512 | 512 | ✅ PASS |
| 100GB | 102400 | 102400 | ✅ PASS |
| 8000MB | 8000 | 8000 | ✅ PASS |
| 1T | 1048576 | 1048576 | ✅ PASS |
| 2TB | 2097152 | 2097152 | ✅ PASS |
| 500G | 512000 | 512000 | ✅ PASS |
| invalid | ERROR | ERROR | ✅ PASS |

---

### Test 2: Partition Validation (valid) ✅
| Partition | Status |
|-----------|--------|
| cpu48c | ✅ PASS |
| cpu52c | ✅ PASS |
| cpu112c | ✅ PASS |
| fat | ✅ PASS |
| gpu | ✅ PASS |

---

### Test 3: Partition Validation (invalid) ✅
| Partition | Expected | Actual | Status |
|-----------|----------|--------|--------|
| nonexistent_partition_xyz | ERROR | ERROR | ✅ PASS |
| "" (empty) | ERROR | ERROR | ✅ PASS |

---

### Test 4: Node Resource Validation ✅

| Partition | Cores | Memory | Expected | Status | Notes |
|-----------|-------|--------|----------|--------|-------|
| cpu48c | 4 | 8G | PASS | ✅ | comput1 has 48 idle cores, 763GB |
| cpu48c | 48 | 100G | PASS | ✅ | Full node available |
| cpu112c | 10 | 100G | FAIL* | ✅ | No node with 10+ idle cores (cluster busy) |
| cpu112c | 112 | 200G | FAIL* | ✅ | No node with 112+ idle cores |
| fat | 1 | 1G | PASS | ✅ | comput5 has resources |
| fat | 200 | 500G | FAIL* | ✅ | Only 96 cores in fat partition |
| cpu48c | 1000 | 10T | FAIL | ✅ | No such large node exists |

*Expected failures due to cluster load/partition specs

---

### Test 5: Edge Cases ✅
| Test Case | Expected | Actual | Status |
|-----------|----------|--------|--------|
| Empty partition | ERROR | ERROR | ✅ PASS |
| Invalid memory format | ERROR | ERROR | ✅ PASS |
| Zero cores | ERROR | ERROR | ✅ PASS |

---

## Code Changes Summary

### Files Modified

1. **internal/engine/slurm.go**
   - Line 29-30: Added empty partition validation
   - Line 30-37: Changed from `cmd.Run()` to `cmd.Output()` with empty check
   - Line 47-50: Added zero/negative cores validation
   - Line 96-97: Changed from `strings.Fields()` to `strings.Split()`

2. **internal/engine/system.go**
   - Line 103: Updated comment to include TB
   - Line 145-146: Added TB/TERABYTE/TERABYTES case

3. **internal/engine/system_test.go**
   - Existing tests remain compatible
   - SLURM tests skipped without SLURM environment

---

## Verification Steps

To verify SLURM resource validation:

```bash
# 1. Build the project
cd ~/xdxtools
source ~/miniconda3/bin/activate node
go build -o xdxtools

# 2. Run validation tests
cd test_slurm
go run main.go

# 3. Run unit tests
cd ~/xdxtools
go test ./internal/engine/ -v

# 4. Manual verification
ssh login2
sinfo -N -h -p cpu48c -o "%n,%T,%C,%m"
```

---

## Conclusion

All critical bugs have been fixed and SLURM resource validation is now working correctly on the login2 cluster. The validation properly:

1. ✅ Parses memory in various formats (M, MB, G, GB, T, TB)
2. ✅ Validates partition existence
3. ✅ Checks node resources (cores and memory)
4. ✅ Handles invalid inputs gracefully
5. ✅ Provides clear error messages

The test suite created in `test_slurm/main.go` can be used for regression testing.
