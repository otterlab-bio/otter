package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

func TestAppendReferencePromotionAuditPersistsPreviousAndPromotedLocks(t *testing.T) {
	projectRoot := t.TempDir()
	previousLock := configv1.ReferencesLock{
		SchemaVersion: configv1.ReferencesLockSchemaVersion,
		References: map[string]configv1.LockedReference{
			"primary": {
				ID:             "hg38",
				Release:        "GRCh38.p13",
				ManifestDigest: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
			},
		},
	}
	promotedLock := configv1.ReferencesLock{
		SchemaVersion:     configv1.ReferencesLockSchemaVersion,
		PromotedFromRunID: "run-20260727T000000Z-abcdef",
		References: map[string]configv1.LockedReference{
			"primary": {
				ID:             "hg38",
				Release:        "GRCh38.p14",
				ManifestDigest: "sha256:2222222222222222222222222222222222222222222222222222222222222222",
			},
		},
	}
	lockPath := filepath.Join(projectRoot, "references.lock.yaml")
	if err := appendReferencePromotionAudit(projectRoot, lockPath, previousLock, promotedLock, promotedLock.PromotedFromRunID); err != nil {
		t.Fatal(err)
	}

	auditPath := filepath.Join(projectRoot, referencePromotionAuditRelativePath)
	auditData, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var records []referencePromotionAuditRecord
	for _, line := range strings.Split(strings.TrimSpace(string(auditData)), "\n") {
		var record referencePromotionAuditRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	if len(records) != 1 {
		t.Fatalf("expected exactly one promotion audit record, got %#v", records)
	}
	record := records[0]
	if record.Action != "reference_promote" || record.RunID != promotedLock.PromotedFromRunID || record.LockPath != lockPath {
		t.Fatalf("unexpected promotion audit identity: %#v", record)
	}
	if record.PreviousLock.References["primary"].Release != "GRCh38.p13" || record.PromotedLock.References["primary"].Release != "GRCh38.p14" {
		t.Fatalf("promotion audit did not preserve lock transition: %#v", record)
	}
}
