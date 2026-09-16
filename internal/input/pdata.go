package input

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/360EntSecGroup-Skylar/excelize"
	"github.com/otterlab-bio/otter/internal/logger"
)

// PDataParser parses phenotype data files (CSV/Excel)
type PDataParser struct{}

// NewPDataParser creates a new pdata parser
func NewPDataParser() *PDataParser {
	return &PDataParser{}
}

// Load loads a pdata file (CSV or Excel)
func (p *PDataParser) Load(filePath string) (*PData, error) {
	logger.Infof("Loading pdata file: %s", filePath)

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("pdata file not found: %s", filePath)
	}

	// Determine file type by extension
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".csv":
		return p.loadCSV(filePath)
	case ".xlsx", ".xls":
		return p.loadExcel(filePath)
	default:
		return nil, fmt.Errorf("unsupported file format: %s (supported: .csv, .xlsx)", ext)
	}
}

// loadExcel loads an Excel file (.xlsx or .xls)
func (p *PDataParser) loadExcel(filePath string) (*PData, error) {
	// Open Excel file
	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open Excel file: %w", err)
	}

	// Get the first sheet using GetSheetMap
	sheetMap := f.GetSheetMap()
	if len(sheetMap) == 0 {
		return nil, fmt.Errorf("Excel file has no sheets")
	}
	sheetName := sheetMap[1] // First sheet
	if sheetName == "" {
		return nil, fmt.Errorf("Excel file has no sheets")
	}

	// Read all rows from the first sheet
	rows := f.GetRows(sheetName)

	if len(rows) == 0 {
		return nil, fmt.Errorf("Excel sheet is empty")
	}

	// Extract columns from first row
	columns := rows[0]
	normalizedColumns, sampleIDCol, aliasCount := normalizePDataColumns(columns)
	if sampleIDCol == -1 {
		logger.Warn("No 'sampleid' column found, will use row indices as sample IDs")
	}

	data, samples, err := buildPDataRecords(rows[1:], columns, normalizedColumns, sampleIDCol, aliasCount)
	if err != nil {
		return nil, err
	}

	logger.Infof("Loaded %d samples from Excel file", len(samples))

	return &PData{
		Samples: samples,
		Columns: columns,
		Data:    data,
	}, nil
}

// loadCSV loads a CSV file
func (p *PDataParser) loadCSV(filePath string) (*PData, error) {
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	// Create CSV reader
	reader := csv.NewReader(file)

	// Read header
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV file: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("CSV file is empty")
	}

	// Extract columns
	columns := records[0]
	normalizedColumns, sampleIDCol, aliasCount := normalizePDataColumns(columns)
	if sampleIDCol == -1 {
		logger.Warn("No 'sampleid' column found, will use row indices as sample IDs")
	}

	data, samples, err := buildPDataRecords(records[1:], columns, normalizedColumns, sampleIDCol, aliasCount)
	if err != nil {
		return nil, err
	}

	logger.Infof("Loaded %d samples from pdata file", len(samples))

	return &PData{
		Samples: samples,
		Columns: columns,
		Data:    data,
	}, nil
}

func normalizePDataColumns(columns []string) ([]string, int, int) {
	normalizedColumns := make([]string, len(columns))
	sampleIDCol := -1
	aliasCount := 0

	for i, col := range columns {
		normalized := col
		switch col {
		case "样本编号", "样本ID", "sample_id":
			normalized = "sampleid"
		case "样本分组", "分组", "group":
			normalized = "sample_group"
		case "条件", "treatment", "condition":
			normalized = "condition"
		}

		normalizedColumns[i] = normalized
		if normalized != col {
			aliasCount++
		}
		if sampleIDCol == -1 && strings.EqualFold(normalized, "sampleid") {
			sampleIDCol = i
		}
	}

	return normalizedColumns, sampleIDCol, aliasCount
}

func buildPDataRecords(records [][]string, columns, normalizedColumns []string, sampleIDCol, aliasCount int) (map[string]map[string]string, []string, error) {
	data := make(map[string]map[string]string, len(records))
	samples := make([]string, 0, len(records))

	for rowOffset, record := range records {
		rowNumber := rowOffset + 2
		sampleID := fmt.Sprintf("Sample%d", rowOffset+1)
		if sampleIDCol >= 0 {
			if sampleIDCol >= len(record) {
				return nil, nil, fmt.Errorf("pdata row %d is missing the sampleid column", rowNumber)
			}
			sampleID = strings.TrimSpace(record[sampleIDCol])
			if sampleID == "" {
				return nil, nil, fmt.Errorf("pdata row %d has an empty sampleid", rowNumber)
			}
		}
		if _, exists := data[sampleID]; exists {
			return nil, nil, fmt.Errorf("pdata contains duplicate sampleid %q at row %d", sampleID, rowNumber)
		}

		sampleData := make(map[string]string, len(columns)+aliasCount)
		for columnIndex, value := range record {
			if columnIndex >= len(columns) {
				break
			}

			normalizedColumnName := normalizedColumns[columnIndex]
			sampleData[normalizedColumnName] = value
			if normalizedColumnName != columns[columnIndex] {
				sampleData[columns[columnIndex]] = value
			}
		}

		samples = append(samples, sampleID)
		data[sampleID] = sampleData
	}

	return data, samples, nil
}

// Validate validates pdata against sample list
func (p *PDataParser) Validate(pdata *PData, samples []string) []string {
	var errors []string

	// Check if all samples in sample list have pdata
	for _, sample := range samples {
		if _, ok := pdata.Data[sample]; !ok {
			errors = append(errors, fmt.Sprintf("sample '%s' not found in pdata", sample))
		}
	}

	// Check for required columns
	requiredCols := []string{"sampleid"}
	for _, requiredCol := range requiredCols {
		found := false
		for _, col := range pdata.Columns {
			if strings.ToLower(col) == requiredCol {
				found = true
				break
			}
		}
		if !found {
			logger.Warnf("Recommended column '%s' not found in pdata", requiredCol)
		}
	}

	if len(errors) > 0 {
		logger.Errorf("PData validation failed with %d errors", len(errors))
	} else {
		logger.Info("PData validation passed")
	}

	return errors
}

// Merge merges pdata with sample information
func (p *PDataParser) Merge(samples []Sample, pdata *PData) []Sample {
	for i := range samples {
		if sampleData, ok := pdata.Data[samples[i].Name]; ok {
			if samples[i].Metadata == nil {
				samples[i].Metadata = make(map[string]string)
			}
			// Merge pdata into metadata
			for key, value := range sampleData {
				samples[i].Metadata[key] = value
			}
		}
	}
	return samples
}
