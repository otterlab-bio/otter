# xdxtools Test Data Directory

This directory contains all test data, configurations, and sample projects for testing the xdxtools Go package.

## Directory Structure

```
testdata/
├── README.md              # This file
├── fastq/                 # FASTQ test files
│   └── test_fastq/       # Sample FASTQ files (3 samples, 6 files total)
│       ├── sample1_R1.fastq.gz
│       ├── sample1_R2.fastq.gz
│       ├── sample2_R1.fastq.gz
│       ├── sample2_R2.fastq.gz
│       ├── sample3_R1.fastq.gz
│       └── sample3_R2.fastq.gz
│
├── pdata/                 # Phenotype data test files
│   ├── test_pdata.csv              # Basic pdata with barcodes
│   ├── test_pdata_chinese.csv     # Chinese column names
│   ├── test_pdata_mixed.csv       # Mixed columns
│   ├── test_pdata_no_condition.csv # No condition field
│   └── test_pdata_single.csv      # Single sample
│
└── projects/              # Sample generated projects
    └── sample_projects/   # 13 different test scenarios
        ├── test_adapter/          # Adapter generation test
        ├── test_barcode_alias/   # Barcode alias test
        ├── test_chinese_columns/ # Chinese columns test
        ├── test_complete_fields/ # Complete fields test
        ├── test_group_levels/    # Group levels calculation
        ├── test_group_pdata/     # Group pdata test
        ├── test_job1/            # Basic job test
        ├── test_missing_fields/   # Missing fields test
        ├── test_mixed_columns/   # Mixed columns test
        ├── test_no_condition/    # No condition test
        ├── test_pdx/             # PDX mode test
        ├── test_single_group/    # Single group test
        └── test_with_pdata/      # With pdata test
```

## Usage

### Running Tests with Test Data

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -v ./internal/input -run TestAdapterGenerator
```

### Using Test Data in Manual Tests

```bash
# Test with specific pdata file
xdxtools create \
    --fastq testdata/fastq/test_fastq \
    --pdata testdata/pdata/test_pdata.csv \
    --mode RRBS \
    --output testdata/projects

# Test with Chinese pdata
xdxtools create \
    --fastq testdata/fastq/test_fastq \
    --pdata testdata/pdata/test_pdata_chinese.csv \
    --mode RRBS \
    --output testdata/projects

# Test PDX mode
xdxtools create \
    --fastq testdata/fastq/test_fastq \
    --mode RRBS \
    --species1 human \
    --species2 mouse \
    --output testdata/projects
```

### Validating Generated Configurations

```bash
# Validate a generated config
xdxtools config validate \
    --config testdata/projects/sample_projects/test_pdx/config/config.yaml
```

## Test Scenarios Covered

1. **Basic Functionality**
   - FASTQ file scanning and pairing
   - Sample name extraction
   - Basic configuration generation

2. **PDX Mode**
   - Dual species configuration (human + mouse)
   - PDX-specific directory structure
   - Workflow name: BeaverPDX

3. **Barcode Processing**
   - Barcode detection and parsing
   - Adapter generation with reverse complement
   - NO_ADAPTER_CAL_USE_DEFAULT handling

4. **Column Name Mapping**
   - English column names (sampleid, condition, etc.)
   - Chinese column names (样本编号 → sampleid, 条件 → condition)
   - Automatic column standardization

5. **Group Levels**
   - Automatic group calculation from pdata
   - Priority: sample_group > condition
   - Single group handling

6. **Excel Support**
   - .xlsx format support
   - .xls format support
   - CSV format compatibility

7. **Error Handling**
   - Missing FASTQ files
   - Invalid sample pairing
   - Configuration validation

## Sample Project Structure

Each project in `projects/sample_projects/` contains:

```
project_name/
├── config/
│   └── config.yaml          # Generated configuration
├── data/                   # Data directory (empty in tests)
├── log/                    # Log directory
├── analysis/               # Analysis results directory
└── workflow/               # Workflow output directory
```

## Notes

- FASTQ files in `fastq/test_fastq/` are empty files (0 bytes) used for path testing
- Actual sequence data is not included to keep the repository size small
- For real data testing, replace these files with actual FASTQ files
- All sample projects were generated using actual `xdxtools create` commands
- Configuration files are real and can be used for validation

## Maintenance

To add new test scenarios:

1. Create new pdata file in `pdata/`
2. Run `xdxtools create` with new parameters
3. Move generated project to `projects/sample_projects/`
4. Update this README if needed

## Requirements

- Go 1.21+
- xdxtools binary (built from source)
- No external dependencies required for basic testing
