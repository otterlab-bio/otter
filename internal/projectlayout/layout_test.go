package projectlayout

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectReportsTrackFromMarkers(t *testing.T) {
	testCases := []struct {
		name          string
		v1Marker      bool
		legacyMarker  bool
		expectedTrack Track
		expectError   bool
	}{
		{name: "no marker", expectedTrack: TrackNone},
		{name: "v1 marker", v1Marker: true, expectedTrack: TrackV1},
		{name: "legacy marker", legacyMarker: true, expectedTrack: TrackLegacy},
		{name: "both markers", v1Marker: true, legacyMarker: true, expectError: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			if testCase.v1Marker {
				writeMarker(t, ProjectLockPath(projectRoot), "schema_version: "+ProjectLockSchemaVersion+"\nproject_id: p\n")
			}
			if testCase.legacyMarker {
				writeMarker(t, LegacyManifestPath(projectRoot), "{}\n")
			}
			track, err := Detect(projectRoot)
			if testCase.expectError {
				if err == nil {
					t.Fatalf("expected both markers to be ambiguous, got track %q", track)
				}
				if !strings.Contains(err.Error(), "contains both") {
					t.Fatalf("expected an ambiguity error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Detect failed: %v", err)
			}
			if track != testCase.expectedTrack {
				t.Fatalf("Detect = %q, want %q", track, testCase.expectedTrack)
			}
		})
	}
}

func TestRequireTrackPointsAtTheProducingCommand(t *testing.T) {
	projectRoot := t.TempDir()
	if err := RequireTrack(projectRoot, TrackV1); err == nil || !strings.Contains(err.Error(), "otter init") {
		t.Fatalf("expected the missing v1 marker to name the producing command, got %v", err)
	}

	// Legacy authoring has been removed, so requiring the legacy track is an
	// internal error rather than a user hint.
	if err := RequireTrack(projectRoot, TrackLegacy); err == nil || !strings.Contains(err.Error(), "unsupported required track") {
		t.Fatalf("expected requiring the legacy track to be unsupported, got %v", err)
	}

	// A pre-existing legacy project is routed at migration, not at authoring.
	writeMarker(t, LegacyManifestPath(projectRoot), "{}\n")
	if err := RequireTrack(projectRoot, TrackV1); err == nil || !strings.Contains(err.Error(), "otter config migrate") {
		t.Fatalf("expected the legacy project to be routed at config migrate, got %v", err)
	}
}

func TestWriteProjectLockRefusesOverwriteAndRoundTrips(t *testing.T) {
	projectRoot := t.TempDir()
	lock := ProjectLock{
		ProjectID: "cohort-a",
		Assets: []AssetSet{
			{Role: "workflows", Source: "inst/snakefiles", Digest: "sha256:aaaa", Files: 23},
		},
	}
	path, err := WriteProjectLock(projectRoot, lock)
	if err != nil {
		t.Fatalf("WriteProjectLock failed: %v", err)
	}
	if path != ProjectLockPath(projectRoot) {
		t.Fatalf("lock written to %q, want %q", path, ProjectLockPath(projectRoot))
	}

	loaded, err := ReadProjectLock(projectRoot)
	if err != nil {
		t.Fatalf("ReadProjectLock failed: %v", err)
	}
	if loaded.SchemaVersion != ProjectLockSchemaVersion {
		t.Fatalf("schema version was not stamped: %q", loaded.SchemaVersion)
	}
	if loaded.ProjectID != "cohort-a" || len(loaded.Assets) != 1 || loaded.Assets[0].Files != 23 {
		t.Fatalf("lock did not round-trip: %+v", loaded)
	}

	if _, err := WriteProjectLock(projectRoot, lock); err == nil {
		t.Fatal("expected a second WriteProjectLock to be refused so pinned digests are not lost")
	}
}

func TestWriteProjectLockRequiresProjectID(t *testing.T) {
	if _, err := WriteProjectLock(t.TempDir(), ProjectLock{}); err == nil {
		t.Fatal("expected an empty project id to be rejected")
	}
}

func TestReadProjectLockRejectsForeignSchema(t *testing.T) {
	projectRoot := t.TempDir()
	writeMarker(t, ProjectLockPath(projectRoot), "schema_version: otter.project/v1\nproject_id: wrong\n")
	if _, err := ReadProjectLock(projectRoot); err == nil {
		t.Fatal("expected a foreign schema version to be rejected")
	}
}

func writeMarker(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
