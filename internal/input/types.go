package input

import "time"

// Sample represents a FASTQ sample
type Sample struct {
	Name      string            `json:"name"`
	FastqR1   string            `json:"fastq_r1"`
	FastqR2   string            `json:"fastq_r2,omitempty"`
	Metadata  map[string]string `json:"metadata"`
	CreatedAt time.Time         `json:"created_at"`
}

// PairedSample represents a paired-end sample
type PairedSample struct {
	Name   string `json:"name"`
	R1Path string `json:"r1_path"`
	R2Path string `json:"r2_path"`
	Valid  bool   `json:"valid"`
}

// PData represents phenotype data
type PData struct {
	Samples []string                     `json:"samples"`
	Columns []string                     `json:"columns"`
	Data    map[string]map[string]string `json:"data"`
}

// ScanOptions represents options for FASTQ scanning
type ScanOptions struct {
	FastqDir   string
	Suffix1    string
	Suffix2    string
	Extensions []string
	Recursive  bool
}

// PairOptions represents options for sample pairing
type PairOptions struct {
	StrictMatching bool
	AllowSingleEnd bool
	MinFiles       int
}

// InputValidationResult represents the result of input validation
type InputValidationResult struct {
	Valid         bool
	Errors        []string
	Warnings      []string
	PairedSamples []PairedSample
	PData         *PData
}
