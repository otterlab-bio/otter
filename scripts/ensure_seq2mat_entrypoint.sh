#!/usr/bin/env bash
set -euo pipefail

TARGET_DIR="${1:-seq2mat}"
ENTRYPOINT="${TARGET_DIR}/cmd/seq2mat/main.go"

if [ -f "${ENTRYPOINT}" ]; then
  echo "seq2mat entrypoint exists: ${ENTRYPOINT}"
  exit 0
fi

echo "seq2mat entrypoint missing, generating fallback at ${ENTRYPOINT}"
mkdir -p "$(dirname "${ENTRYPOINT}")"

cat > "${ENTRYPOINT}" <<'EOF'
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/otterlab-bio/seq2mat/internal/database"
	"github.com/otterlab-bio/seq2mat/internal/htseq"
	"github.com/otterlab-bio/seq2mat/internal/processor"
)

var (
	htseqDir  = flag.String("htseq_dir", "", "Input directory containing HTSeq files (required)")
	postfix   = flag.String("postfix", "_human.txt", "File suffix pattern (default: _human.txt)")
	outputDir = flag.String("output_dir", "", "Output directory for results (required)")
	showVer   = flag.Bool("version", false, "Print version")
)

var Version = "0.1.0"

func main() {
	flag.Parse()

	if *showVer {
		fmt.Printf("seq2mat %s\n", Version)
		return
	}
	if *htseqDir == "" {
		log.Fatal("--htseq_dir is required")
	}
	if *outputDir == "" {
		log.Fatal("--output_dir is required")
	}
	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	species := database.DetectSpecies(*postfix)

	var geneDB database.GeneDatabase
	if database.CheckAvailable() {
		db := database.NewEmbeddedDatabase()
		if err := db.LoadBothSpecies(""); err != nil {
			return fmt.Errorf("failed to load embedded database: %w", err)
		}
		geneDB = db
	} else {
		dbPath := "internal/database"
		db := database.NewCSVDatabase()
		if err := db.LoadBothSpecies(dbPath); err != nil {
			return fmt.Errorf("failed to load CSV database from %s: %w", dbPath, err)
		}
		geneDB = db
	}
	defer geneDB.Close()

	samples, err := htseq.ReadHTSeqFiles(*htseqDir, *postfix)
	if err != nil {
		return fmt.Errorf("failed to read HTSeq files: %w", err)
	}

	df, err := processor.MergeSamples(samples)
	if err != nil {
		return fmt.Errorf("failed to merge HTSeq files: %w", err)
	}

	var conversionStatistics processor.GeneIDConversionStats
	df, conversionStatistics, err = processor.ConvertGeneIDs(df, geneDB, species)
	if err != nil {
		return fmt.Errorf("failed to convert gene IDs: %w", err)
	}

	df = processor.AggregateDuplicates(df)
	df = processor.FilterInvalidRows(df)
	normalized := processor.Normalize(df)

	if err := os.MkdirAll(*outputDir, 0o755); err != nil {
		return fmt.Errorf("failed to create output dir: %w", err)
	}

	if err := df.WriteTSV(filepath.Join(*outputDir, "matrix_count.txt")); err != nil {
		return fmt.Errorf("failed to write matrix_count.txt: %w", err)
	}
	if err := normalized.WriteTSV(filepath.Join(*outputDir, "matrix_norm.txt")); err != nil {
		return fmt.Errorf("failed to write matrix_norm.txt: %w", err)
	}

	fmt.Printf("Successfully processed %d samples\n", len(samples))
	fmt.Printf("Output written to %s\n", *outputDir)
	return nil
}
EOF

echo "Generated fallback seq2mat entrypoint."
