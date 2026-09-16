package artifact

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"hash/crc32"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/360EntSecGroup-Skylar/excelize"
)

func TestComparatorRegistryComparesScientificExpressionMatrices(t *testing.T) {
	leftResultsRoot := t.TempDir()
	rightResultsRoot := t.TempDir()
	writeComparisonArtifact(t, leftResultsRoot, "methylation/matrix_count.txt", "gene\tsample-a\ngene-a\t12\ngene-b\t4\n")
	writeComparisonArtifact(t, rightResultsRoot, "methylation/matrix_count.txt", "gene\tsample-a\ngene-a\t12.0000005\ngene-b\t4\n")

	leftManifest := validManifest(checksum([]byte("left")))
	leftManifest.Artifacts[0] = Entry{
		ID:        "expression-count-matrix",
		Path:      "methylation/matrix_count.txt",
		MediaType: "text/tab-separated-values",
		Schema:    "otter.expression-count-matrix/v1",
		Checksum:  checksum([]byte("left")),
		Comparison: Comparison{
			Tier:       ComparisonTierScientific,
			Comparator: ComparatorExpressionMatrix,
		},
	}
	rightManifest := leftManifest
	rightManifest.Artifacts = append([]Entry(nil), leftManifest.Artifacts...)
	rightManifest.Artifacts[0].Checksum = checksum([]byte("right"))

	report, err := NewComparatorRegistry().Compare(leftResultsRoot, leftManifest, rightResultsRoot, rightManifest)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || !report.Comparable || len(report.Issues) != 0 {
		t.Fatalf("expected tolerant expression matrix comparison to pass, got %#v", report)
	}
}

func TestComparatorRegistryFailsClosedForUnknownAndInvalidMethrixComparators(t *testing.T) {
	leftResultsRoot := t.TempDir()
	rightResultsRoot := t.TempDir()
	writeComparisonArtifact(t, leftResultsRoot, "methylation/matrix.h5", "not-an-hdf5-file")
	writeComparisonArtifact(t, rightResultsRoot, "methylation/matrix.h5", "not-an-hdf5-file")

	leftManifest := validManifest(checksum([]byte("left")))
	rightManifest := validManifest(checksum([]byte("right")))
	report, err := NewComparatorRegistry().Compare(leftResultsRoot, leftManifest, rightResultsRoot, rightManifest)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || len(report.Issues) != 1 || !strings.Contains(report.Issues[0].Details, "does not have an HDF5 signature") {
		t.Fatalf("expected invalid Methrix HDF5 artifact to fail closed, got %#v", report)
	}

	leftManifest.Artifacts[0].Comparison.Comparator = "unregistered/v1"
	rightManifest.Artifacts[0].Comparison.Comparator = "unregistered/v1"
	report, err = NewComparatorRegistry().Compare(leftResultsRoot, leftManifest, rightResultsRoot, rightManifest)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || report.Comparable || len(report.Issues) != 1 || !strings.Contains(report.Issues[0].Details, "no registered implementation") {
		t.Fatalf("expected unknown comparator to fail closed, got %#v", report)
	}
}

func TestComparatorRegistryValidatesPDXClassificationSemantics(t *testing.T) {
	leftResultsRoot := t.TempDir()
	rightResultsRoot := t.TempDir()
	classification := "artifact_index\tsource_bam\tsource_bai\tmapped_reads\n0001\tsample_Filtered.bam\tsample_Filtered.bam.bai\t17\n"
	writeComparisonArtifact(t, leftResultsRoot, "pdx/graft/classification.tsv", classification)
	writeComparisonArtifact(t, rightResultsRoot, "pdx/graft/classification.tsv", classification)

	leftManifest := validManifest(checksum([]byte("left")))
	leftManifest.Artifacts[0] = Entry{
		ID:        "graft-classification-summary",
		Path:      "pdx/graft/classification.tsv",
		MediaType: "text/tab-separated-values",
		Schema:    "otter.pdx-graft-classification/v1",
		Checksum:  checksum([]byte("left")),
		Comparison: Comparison{
			Tier:       ComparisonTierScientific,
			Comparator: ComparatorPDXGraftReadCounts,
		},
	}
	leftManifest.Artifacts = append(leftManifest.Artifacts,
		Entry{
			ID:         "graft-alignment-bam-0001",
			Path:       "pdx/graft/sample_Filtered.bam",
			MediaType:  "application/x-bam",
			Schema:     "sam-bam/v1",
			Checksum:   checksum([]byte("bam")),
			Comparison: Comparison{Tier: ComparisonTierExact, Comparator: ComparatorExactFile},
		},
		Entry{
			ID:         "graft-alignment-bai-0001",
			Path:       "pdx/graft/sample_Filtered.bam.bai",
			MediaType:  "application/x-bam-index",
			Schema:     "sam-bai/v1",
			Checksum:   checksum([]byte("bai")),
			Comparison: Comparison{Tier: ComparisonTierExact, Comparator: ComparatorExactFile},
		},
	)
	rightManifest := leftManifest
	rightManifest.Artifacts = append([]Entry(nil), leftManifest.Artifacts...)
	rightManifest.Artifacts[0].Checksum = checksum([]byte("right"))

	report, err := NewComparatorRegistry().Compare(leftResultsRoot, leftManifest, rightResultsRoot, rightManifest)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed {
		t.Fatalf("expected PDX classification comparison to pass, got %#v", report)
	}
	writeComparisonArtifact(t, rightResultsRoot, "pdx/graft/classification.tsv", "artifact_index\tsource_bam\tsource_bai\tmapped_reads\n0001\tother_Filtered.bam\tother_Filtered.bam.bai\t17\n")
	report, err = NewComparatorRegistry().Compare(leftResultsRoot, leftManifest, rightResultsRoot, rightManifest)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || len(report.Issues) != 1 || !strings.Contains(report.Issues[0].Details, "does not match declared BAM/BAI pair") {
		t.Fatalf("expected undeclared PDX BAM reference to fail, got %#v", report)
	}
	writeComparisonArtifact(t, rightResultsRoot, "pdx/graft/classification.tsv", "artifact_index\tsource_bam\tsource_bai\tmapped_reads\n0001\tsample_Filtered.bam\tsample_Filtered.bam.bai\t0\n")
	report, err = NewComparatorRegistry().Compare(leftResultsRoot, leftManifest, rightResultsRoot, rightManifest)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || len(report.Issues) != 1 || !strings.Contains(report.Issues[0].Details, "invalid mapped_reads") {
		t.Fatalf("expected invalid PDX mapped reads to fail, got %#v", report)
	}
}

func TestComparatorRegistryComparesTypedRNASplicingOutcomesAcrossSourceRoots(t *testing.T) {
	leftResultsRoot := t.TempDir()
	rightResultsRoot := t.TempDir()
	writeComparisonArtifact(t, leftResultsRoot, "splicing/outcome.json", `{
  "schema_version": "otter.rna-splicing-outcome/v1",
  "status": "produced",
  "source_root": "/left-run/work/splicing",
  "artifact_paths": ["events.tsv"]
}`)
	writeComparisonArtifact(t, rightResultsRoot, "splicing/outcome.json", `{
  "schema_version": "otter.rna-splicing-outcome/v1",
  "status": "produced",
  "source_root": "/right-run/work/splicing",
  "artifact_paths": ["events.tsv"]
}`)
	writeComparisonArtifact(t, leftResultsRoot, "splicing/files/events.tsv", "event\tvalue\nSE\t1\n")
	writeComparisonArtifact(t, rightResultsRoot, "splicing/files/events.tsv", "event\tvalue\nSE\t1\n")

	report, err := NewComparatorRegistry().Compare(leftResultsRoot, rnaSplicingManifest(), rightResultsRoot, rnaSplicingManifest())
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || !report.Comparable || len(report.Issues) != 0 {
		t.Fatalf("expected matching published RNA splicing outcomes to pass despite different source roots, got %#v", report)
	}
}

func TestComparatorRegistryRejectsUnboundTypedRNASplicingOutcome(t *testing.T) {
	leftResultsRoot := t.TempDir()
	rightResultsRoot := t.TempDir()
	for _, resultsRoot := range []string{leftResultsRoot, rightResultsRoot} {
		writeComparisonArtifact(t, resultsRoot, "splicing/outcome.json", `{
  "schema_version": "otter.rna-splicing-outcome/v1",
  "status": "produced",
  "source_root": "/run/work/splicing",
  "artifact_paths": ["unbound-events.tsv"]
}`)
		writeComparisonArtifact(t, resultsRoot, "splicing/files/events.tsv", "event\tvalue\nSE\t1\n")
	}

	report, err := NewComparatorRegistry().Compare(leftResultsRoot, rnaSplicingManifest(), rightResultsRoot, rnaSplicingManifest())
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || len(report.Issues) != 1 || !strings.Contains(report.Issues[0].Details, "does not match declared published output paths") {
		t.Fatalf("expected unbound RNA splicing outcome to fail closed, got %#v", report)
	}
}

func rnaSplicingManifest() Manifest {
	return Manifest{
		SchemaVersion:     SchemaVersion,
		RunID:             "run-20260727T010203Z-abcdef",
		RunSnapshotDigest: checksum([]byte("run snapshot")),
		Scenario:          "rnaseq",
		Toolchain:         "modern",
		Executor:          "craftmake",
		Backend:           "local",
		Artifacts: []Entry{
			{
				ID:         "splicing-outcome",
				Path:       "splicing/outcome.json",
				MediaType:  "application/json",
				Schema:     "otter.rna-splicing-outcome/v1",
				Checksum:   checksum([]byte("outcome")),
				Comparison: Comparison{Tier: ComparisonTierStructural, Comparator: ComparatorRNASplicingOutcome},
			},
			{
				ID:         "splicing-output-0001",
				Path:       "splicing/files/events.tsv",
				MediaType:  "application/octet-stream",
				Schema:     "otter.rna-splicing-output/v1",
				Checksum:   checksum([]byte("events")),
				Comparison: Comparison{Tier: ComparisonTierExact, Comparator: ComparatorExactFile},
			},
		},
	}
}

func TestComparatorRegistryValidatesBAMAndBAIHeaders(t *testing.T) {
	leftResultsRoot := t.TempDir()
	rightResultsRoot := t.TempDir()
	for _, resultsRoot := range []string{leftResultsRoot, rightResultsRoot} {
		writeComparisonArtifactBytes(t, resultsRoot, "pdx/graft/sample.bam", minimalBGZFBAM(t, "chr1"))
		writeComparisonArtifactBytes(t, resultsRoot, "pdx/graft/sample.bam.bai", minimalBAI(1))
	}
	leftManifest := bamPairManifest()
	rightManifest := bamPairManifest()

	report, err := NewComparatorRegistry().Compare(leftResultsRoot, leftManifest, rightResultsRoot, rightManifest)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || !report.Comparable || len(report.Issues) != 0 {
		t.Fatalf("expected valid BAM/BAI pairs to pass, got %#v", report)
	}
}

func TestComparatorRegistryRejectsInvalidBAMAndBAIMetadata(t *testing.T) {
	testCases := []struct {
		name            string
		leftBAM         []byte
		leftBAI         []byte
		expectedDetails string
	}{
		{
			name:            "BAM magic is invalid",
			leftBAM:         minimalBGZFBAMWithMagic(t, "BAD!", "chr1"),
			leftBAI:         minimalBAI(1),
			expectedDetails: "invalid BAM magic",
		},
		{
			name:            "BAI reference count differs",
			leftBAM:         minimalBGZFBAM(t, "chr1"),
			leftBAI:         minimalBAI(2),
			expectedDetails: "reference counts differ",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			leftResultsRoot := t.TempDir()
			rightResultsRoot := t.TempDir()
			writeComparisonArtifactBytes(t, leftResultsRoot, "pdx/graft/sample.bam", testCase.leftBAM)
			writeComparisonArtifactBytes(t, leftResultsRoot, "pdx/graft/sample.bam.bai", testCase.leftBAI)
			writeComparisonArtifactBytes(t, rightResultsRoot, "pdx/graft/sample.bam", minimalBGZFBAM(t, "chr1"))
			writeComparisonArtifactBytes(t, rightResultsRoot, "pdx/graft/sample.bam.bai", minimalBAI(1))

			report, err := NewComparatorRegistry().Compare(leftResultsRoot, bamPairManifest(), rightResultsRoot, bamPairManifest())
			if err != nil {
				t.Fatal(err)
			}
			if report.Passed || len(report.Issues) != 2 {
				t.Fatalf("expected both declared BAM/BAI artifacts to fail, got %#v", report)
			}
			for _, issue := range report.Issues {
				if !strings.Contains(issue.Details, testCase.expectedDetails) {
					t.Fatalf("expected issue to contain %q, got %#v", testCase.expectedDetails, issue)
				}
			}
		})
	}
}

func TestComparatorRegistryRejectsMalformedBAIStructure(t *testing.T) {
	truncatedBinCount := append([]byte(nil), minimalBAI(1)[:8]...)
	negativeBinCount := minimalBAI(1)
	binary.LittleEndian.PutUint32(negativeBinCount[8:12], uint32(0xffffffff))
	excessiveChunkCount := make([]byte, 20)
	copy(excessiveChunkCount, baiMagic)
	binary.LittleEndian.PutUint32(excessiveChunkCount[4:8], 1)
	binary.LittleEndian.PutUint32(excessiveChunkCount[8:12], 1)
	binary.LittleEndian.PutUint32(excessiveChunkCount[16:20], uint32(baiMaximumChunksPerBin+1))
	excessiveLinearOffsetCount := minimalBAI(1)
	binary.LittleEndian.PutUint32(excessiveLinearOffsetCount[12:16], uint32(baiMaximumLinearOffsets+1))
	testCases := []struct {
		name            string
		leftBAI         []byte
		expectedDetails string
	}{
		{
			name:            "truncated reference bin count",
			leftBAI:         truncatedBinCount,
			expectedDetails: "read BAI bin count",
		},
		{
			name:            "negative reference bin count",
			leftBAI:         negativeBinCount,
			expectedDetails: "invalid bin count -1",
		},
		{
			name:            "excessive chunk count",
			leftBAI:         excessiveChunkCount,
			expectedDetails: "invalid chunk count",
		},
		{
			name:            "excessive linear offset count",
			leftBAI:         excessiveLinearOffsetCount,
			expectedDetails: "invalid linear offset count",
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			leftResultsRoot := t.TempDir()
			rightResultsRoot := t.TempDir()
			writeComparisonArtifactBytes(t, leftResultsRoot, "pdx/graft/sample.bam", minimalBGZFBAM(t, "chr1"))
			writeComparisonArtifactBytes(t, leftResultsRoot, "pdx/graft/sample.bam.bai", testCase.leftBAI)
			writeComparisonArtifactBytes(t, rightResultsRoot, "pdx/graft/sample.bam", minimalBGZFBAM(t, "chr1"))
			writeComparisonArtifactBytes(t, rightResultsRoot, "pdx/graft/sample.bam.bai", minimalBAI(1))

			report, err := NewComparatorRegistry().Compare(leftResultsRoot, bamPairManifest(), rightResultsRoot, bamPairManifest())
			if err != nil {
				t.Fatal(err)
			}
			if report.Passed || !report.Comparable || len(report.Issues) != 2 {
				t.Fatalf("expected malformed BAI structure to fail both pair entries, got %#v", report)
			}
			for _, issue := range report.Issues {
				if !strings.Contains(issue.Details, testCase.expectedDetails) {
					t.Fatalf("expected issue to contain %q, got %#v", testCase.expectedDetails, issue)
				}
			}
		})
	}
}

func TestComparatorRegistryRejectsSymbolicLinkBAM(t *testing.T) {
	leftResultsRoot := t.TempDir()
	rightResultsRoot := t.TempDir()
	externalBAMPath := filepath.Join(t.TempDir(), "external.bam")
	if err := os.WriteFile(externalBAMPath, minimalBGZFBAM(t, "chr1"), 0o644); err != nil {
		t.Fatal(err)
	}
	linkedBAMPath := filepath.Join(leftResultsRoot, "pdx", "graft", "sample.bam")
	if err := os.MkdirAll(filepath.Dir(linkedBAMPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalBAMPath, linkedBAMPath); err != nil {
		t.Fatal(err)
	}
	writeComparisonArtifactBytes(t, leftResultsRoot, "pdx/graft/sample.bam.bai", minimalBAI(1))
	writeComparisonArtifactBytes(t, rightResultsRoot, "pdx/graft/sample.bam", minimalBGZFBAM(t, "chr1"))
	writeComparisonArtifactBytes(t, rightResultsRoot, "pdx/graft/sample.bam.bai", minimalBAI(1))

	report, err := NewComparatorRegistry().Compare(leftResultsRoot, bamPairManifest(), rightResultsRoot, bamPairManifest())
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || len(report.Issues) != 2 {
		t.Fatalf("expected symbolic-link BAM to fail for both declared artifacts, got %#v", report)
	}
	for _, issue := range report.Issues {
		if !strings.Contains(issue.Details, "non-empty regular file") {
			t.Fatalf("expected symbolic-link rejection, got %#v", issue)
		}
	}
}

func bamPairManifest() Manifest {
	return Manifest{
		SchemaVersion:     SchemaVersion,
		RunID:             "run-20260727T010203Z-abcdef",
		RunSnapshotDigest: checksum([]byte("run snapshot")),
		Scenario:          "bs-pdx",
		Toolchain:         "modern",
		Executor:          "craftmake",
		Backend:           "local",
		Artifacts: []Entry{
			{
				ID:         "graft-alignment-bam-0001",
				Path:       "pdx/graft/sample.bam",
				MediaType:  "application/x-bam",
				Schema:     "sam-bam/v1",
				Checksum:   checksum([]byte("bam")),
				Comparison: Comparison{Tier: ComparisonTierStructural, Comparator: ComparatorBAMPairStructure},
			},
			{
				ID:         "graft-alignment-bai-0001",
				Path:       "pdx/graft/sample.bam.bai",
				MediaType:  "application/x-bam-index",
				Schema:     "sam-bai/v1",
				Checksum:   checksum([]byte("bai")),
				Comparison: Comparison{Tier: ComparisonTierStructural, Comparator: ComparatorBAMPairStructure},
			},
		},
	}
}

func minimalBGZFBAM(t *testing.T, referenceName string) []byte {
	t.Helper()
	return minimalBGZFBAMWithMagic(t, bamMagic, referenceName)
}

func minimalBGZFBAMWithMagic(t *testing.T, magic string, referenceName string) []byte {
	t.Helper()
	uncompressedBAM := bytes.NewBufferString(magic)
	writeLittleEndianInt32(t, uncompressedBAM, 0)
	writeLittleEndianInt32(t, uncompressedBAM, 1)
	writeLittleEndianInt32(t, uncompressedBAM, int32(len(referenceName)+1))
	uncompressedBAM.WriteString(referenceName)
	uncompressedBAM.WriteByte(0)
	writeLittleEndianInt32(t, uncompressedBAM, 1)

	var compressedPayload bytes.Buffer
	deflater, err := flate.NewWriter(&compressedPayload, flate.DefaultCompression)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := deflater.Write(uncompressedBAM.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := deflater.Close(); err != nil {
		t.Fatal(err)
	}
	blockSize := 18 + compressedPayload.Len() + 8
	if blockSize > 65_536 {
		t.Fatalf("minimal BGZF block is too large: %d", blockSize)
	}
	bgzfBlock := make([]byte, 0, blockSize)
	bgzfBlock = append(bgzfBlock, 0x1f, 0x8b, 0x08, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0xff, 0x06, 0x00, 'B', 'C', 0x02, 0x00)
	blockSizeMinusOne := make([]byte, 2)
	binary.LittleEndian.PutUint16(blockSizeMinusOne, uint16(blockSize-1))
	bgzfBlock = append(bgzfBlock, blockSizeMinusOne...)
	bgzfBlock = append(bgzfBlock, compressedPayload.Bytes()...)
	crc := make([]byte, 4)
	binary.LittleEndian.PutUint32(crc, crc32.ChecksumIEEE(uncompressedBAM.Bytes()))
	bgzfBlock = append(bgzfBlock, crc...)
	uncompressedSize := make([]byte, 4)
	binary.LittleEndian.PutUint32(uncompressedSize, uint32(uncompressedBAM.Len()))
	return append(bgzfBlock, uncompressedSize...)
}

func minimalBAI(referenceCount uint32) []byte {
	bai := make([]byte, 8, 8+int(referenceCount)*8)
	copy(bai, baiMagic)
	binary.LittleEndian.PutUint32(bai[4:], referenceCount)
	for referenceIndex := uint32(0); referenceIndex < referenceCount; referenceIndex++ {
		bai = append(bai, 0, 0, 0, 0)
		bai = append(bai, 0, 0, 0, 0)
	}
	return bai
}

func writeLittleEndianInt32(t *testing.T, buffer *bytes.Buffer, value int32) {
	t.Helper()
	encodedValue := make([]byte, 4)
	binary.LittleEndian.PutUint32(encodedValue, uint32(value))
	if _, err := buffer.Write(encodedValue); err != nil {
		t.Fatal(err)
	}
}

func TestComparatorRegistryComparesXLSXWorkbookStructure(t *testing.T) {
	leftResultsRoot := t.TempDir()
	rightResultsRoot := t.TempDir()
	writeXLSXComparisonArtifact(t, leftResultsRoot, "qc/qc_summary.xlsx", []string{"Summary", "Quality"})
	writeXLSXComparisonArtifact(t, rightResultsRoot, "qc/qc_summary.xlsx", []string{"Summary", "Quality"})

	leftManifest := xlsxStructureManifest(checksum([]byte("left workbook")))
	rightManifest := xlsxStructureManifest(checksum([]byte("right workbook")))
	report, err := NewComparatorRegistry().Compare(leftResultsRoot, leftManifest, rightResultsRoot, rightManifest)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed || !report.Comparable || len(report.Issues) != 0 {
		t.Fatalf("expected matching XLSX workbook structures to pass, got %#v", report)
	}
}

func TestComparatorRegistryRejectsDifferentXLSXWorkbookSheetTitles(t *testing.T) {
	testCases := []struct {
		name             string
		rightSheetTitles []string
	}{
		{
			name:             "sheet title differs",
			rightSheetTitles: []string{"Summary", "Review"},
		},
		{
			name:             "sheet title order differs",
			rightSheetTitles: []string{"Quality", "Summary"},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			leftResultsRoot := t.TempDir()
			rightResultsRoot := t.TempDir()
			writeXLSXComparisonArtifact(t, leftResultsRoot, "qc/qc_summary.xlsx", []string{"Summary", "Quality"})
			writeXLSXComparisonArtifact(t, rightResultsRoot, "qc/qc_summary.xlsx", testCase.rightSheetTitles)

			leftMembers, err := xlsxMemberNames(filepath.Join(leftResultsRoot, "qc", "qc_summary.xlsx"))
			if err != nil {
				t.Fatal(err)
			}
			rightMembers, err := xlsxMemberNames(filepath.Join(rightResultsRoot, "qc", "qc_summary.xlsx"))
			if err != nil {
				t.Fatal(err)
			}
			if !equalStringSlices(leftMembers, rightMembers) {
				t.Fatalf("fixture must retain matching XLSX package member names: left=%#v right=%#v", leftMembers, rightMembers)
			}

			report, err := NewComparatorRegistry().Compare(
				leftResultsRoot,
				xlsxStructureManifest(checksum([]byte("left workbook"))),
				rightResultsRoot,
				xlsxStructureManifest(checksum([]byte("right workbook"))),
			)
			if err != nil {
				t.Fatal(err)
			}
			if report.Passed || !report.Comparable || len(report.Issues) != 1 || !strings.Contains(report.Issues[0].Details, "sheet titles differ") {
				t.Fatalf("expected different XLSX sheet titles to fail structurally, got %#v", report)
			}
		})
	}
}

func TestComparatorRegistryFailsClosedForInvalidXLSXPackage(t *testing.T) {
	leftResultsRoot := t.TempDir()
	rightResultsRoot := t.TempDir()
	writeComparisonArtifact(t, leftResultsRoot, "qc/qc_summary.xlsx", "not an XLSX package")
	writeComparisonArtifact(t, rightResultsRoot, "qc/qc_summary.xlsx", "not an XLSX package")

	report, err := NewComparatorRegistry().Compare(
		leftResultsRoot,
		xlsxStructureManifest(checksum([]byte("left workbook"))),
		rightResultsRoot,
		xlsxStructureManifest(checksum([]byte("right workbook"))),
	)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || !report.Comparable || len(report.Issues) != 1 || !strings.Contains(report.Issues[0].Details, "open XLSX package") {
		t.Fatalf("expected invalid XLSX package to fail closed, got %#v", report)
	}
}

func xlsxStructureManifest(artifactChecksum string) Manifest {
	manifest := validManifest(artifactChecksum)
	manifest.Artifacts[0] = Entry{
		ID:        "qc-summary",
		Path:      "qc/qc_summary.xlsx",
		MediaType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		Schema:    "otter.qc-summary/v1",
		Checksum:  artifactChecksum,
		Comparison: Comparison{
			Tier:       ComparisonTierStructural,
			Comparator: ComparatorXLSXStructure,
		},
	}
	return manifest
}

func writeXLSXComparisonArtifact(t *testing.T, resultsRoot string, relativePath string, sheetTitles []string) {
	t.Helper()
	if len(sheetTitles) == 0 {
		t.Fatal("XLSX fixture requires at least one sheet title")
	}
	artifactPath := filepath.Join(resultsRoot, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	workbook := excelize.NewFile()
	workbook.SetSheetName("Sheet1", sheetTitles[0])
	for _, sheetTitle := range sheetTitles[1:] {
		workbook.NewSheet(sheetTitle)
	}
	if err := workbook.SaveAs(artifactPath); err != nil {
		t.Fatal(err)
	}
}

func writeComparisonArtifactBytes(t *testing.T, resultsRoot string, relativePath string, content []byte) {
	t.Helper()
	artifactPath := filepath.Join(resultsRoot, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, content, 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeComparisonArtifact(t *testing.T, resultsRoot string, relativePath string, content string) {
	t.Helper()
	artifactPath := filepath.Join(resultsRoot, filepath.FromSlash(relativePath))
	if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(artifactPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
