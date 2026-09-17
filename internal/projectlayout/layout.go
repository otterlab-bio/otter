// Package projectlayout identifies the Otter project track a directory belongs
// to and writes the marker files that make that track explicit.
//
// Otter recognises two project layouts that must not be mixed:
//
//   - The canonical v1 track, marked by project.lock.yaml. It carries
//     project.yaml, samples.tsv, references.lock.yaml, pinned workflow assets,
//     and immutable runs/<run-id>/run.yaml snapshots. This is the only track
//     Otter still authors.
//   - The legacy compatibility layout, marked by .otter/assets.manifest.json.
//     It carries config/otter.yaml under userspace/<jobid>/. Legacy authoring
//     has been removed; the marker is still detected so "otter config migrate",
//     "otter assets", and the run boundary can route pre-existing legacy
//     projects correctly.
//
// The canonical marker is written by "otter init" and is the evidence a later
// command uses to refuse a mixed-track project instead of silently producing a
// project that no executor accepts.
package projectlayout

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	assetspkg "github.com/otterlab-bio/otter/internal/assets"
	"gopkg.in/yaml.v3"
)

// Track identifies which authoring contract a project directory follows.
type Track string

const (
	// TrackNone means no recognised Otter project marker was found.
	TrackNone Track = "none"
	// TrackV1 is the canonical project.yaml/run.yaml track.
	TrackV1 Track = "v1"
	// TrackLegacy is the compatibility otter.yaml track.
	TrackLegacy Track = "legacy"
)

// ProjectLockSchemaVersion is the schema version stamped into project.lock.yaml.
const ProjectLockSchemaVersion = "otter.project.lock/v1"

// ProjectLockFileName is the canonical v1 track marker.
const ProjectLockFileName = "project.lock.yaml"

// AssetSet records one pinned workflow asset directory in project.lock.yaml.
type AssetSet struct {
	// Role is the project-level directory, for example "workflows".
	Role string `yaml:"role"`
	// Source is the packaged origin, for example "inst/snakefiles".
	Source string `yaml:"source"`
	// Digest is the content digest of the pinned directory.
	Digest string `yaml:"digest"`
	// Files is the number of regular files pinned from that source.
	Files int `yaml:"files"`
}

// ProjectLock records the pinned workflow assets and environment declarations of
// a canonical v1 project so a later resolve can detect asset drift.
type ProjectLock struct {
	SchemaVersion string     `yaml:"schema_version"`
	ProjectID     string     `yaml:"project_id"`
	Assets        []AssetSet `yaml:"assets"`
}

// ProjectLockPath returns the project lock path for a project root.
func ProjectLockPath(projectRoot string) string {
	return filepath.Join(projectRoot, ProjectLockFileName)
}

// LegacyManifestPath returns the legacy track marker path for a project root.
func LegacyManifestPath(projectRoot string) string {
	return assetspkg.ManifestPath(projectRoot)
}

// Detect reports which track the project root belongs to.
//
// When both markers exist the directory is ambiguous and an error is returned:
// guessing would let a v1 resolve read legacy assets, or let a legacy run read
// v1 assets, and neither is a state Otter supports.
func Detect(projectRoot string) (Track, error) {
	hasV1 := fileExists(ProjectLockPath(projectRoot))
	hasLegacy := fileExists(LegacyManifestPath(projectRoot))
	switch {
	case hasV1 && hasLegacy:
		return TrackNone, fmt.Errorf(
			"%s contains both %s (canonical v1) and %s (legacy compatibility); remove the stale marker for the track you are not using",
			projectRoot, ProjectLockFileName, filepath.Join(".otter", filepath.Base(LegacyManifestPath(projectRoot))),
		)
	case hasV1:
		return TrackV1, nil
	case hasLegacy:
		return TrackLegacy, nil
	default:
		return TrackNone, nil
	}
}

// RequireTrack fails unless the project root already carries the wanted track
// marker, and points at the command that produces the missing one.
func RequireTrack(projectRoot string, wanted Track) error {
	detected, err := Detect(projectRoot)
	if err != nil {
		return err
	}
	if detected == wanted {
		return nil
	}
	if wanted != TrackV1 {
		return fmt.Errorf("unsupported required track %q", wanted)
	}
	if detected == TrackLegacy {
		return fmt.Errorf(
			"%s is a legacy compatibility project (%s); convert it with `otter config migrate`, or create a canonical project with `otter init <name>` instead",
			projectRoot, filepath.Join(".otter", filepath.Base(LegacyManifestPath(projectRoot))),
		)
	}
	return fmt.Errorf(
		"%s has no %s marker; run `otter init <project>` first so the canonical project assets are pinned",
		projectRoot, ProjectLockFileName,
	)
}

// WriteProjectLock stamps the canonical marker. It refuses to overwrite an
// existing lock so a re-init cannot silently drop pinned asset digests.
func WriteProjectLock(projectRoot string, lock ProjectLock) (string, error) {
	if strings.TrimSpace(lock.ProjectID) == "" {
		return "", fmt.Errorf("project.lock.yaml requires a project id")
	}
	lock.SchemaVersion = ProjectLockSchemaVersion
	path := ProjectLockPath(projectRoot)
	if fileExists(path) {
		return "", fmt.Errorf("%s already exists; delete it explicitly to re-initialise the project", path)
	}
	encoded, err := yaml.Marshal(lock)
	if err != nil {
		return "", fmt.Errorf("marshal project lock: %w", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return "", fmt.Errorf("write project lock %s: %w", path, err)
	}
	return path, nil
}

// ReadProjectLock loads the canonical marker.
func ReadProjectLock(projectRoot string) (ProjectLock, error) {
	path := ProjectLockPath(projectRoot)
	data, err := os.ReadFile(path)
	if err != nil {
		return ProjectLock{}, fmt.Errorf("read project lock %s: %w", path, err)
	}
	var lock ProjectLock
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return ProjectLock{}, fmt.Errorf("parse project lock %s: %w", path, err)
	}
	if lock.SchemaVersion != ProjectLockSchemaVersion {
		return ProjectLock{}, fmt.Errorf("project lock %s has schema_version %q, expected %q", path, lock.SchemaVersion, ProjectLockSchemaVersion)
	}
	return lock, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}
