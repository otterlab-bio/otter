package reference

import (
	"context"

	configv1 "github.com/otterlab-bio/otter/internal/config/v1"
)

const (
	DefaultReferenceRootEnvironmentVariable = "OTTER_REFERENCE_ROOT"
	DefaultReferenceRootDirectory           = ".otter/references"

	ReferenceIndexBismark = "bismark"
	ReferenceIndexBowtie2 = "bowtie2"
	ReferenceIndexSTAR    = "star"
)

// BuildRequest describes one immutable reference-registry release. The input
// assets are copied into a staging directory; source files are never modified.
type BuildRequest struct {
	Context      context.Context
	RegistryRoot string
	ReferenceID  string
	Release      string
	Organism     string
	Assembly     string
	Aliases      []string

	SourceFastaPath string
	SourceGTFPath   string
	Scenarios       []configv1.Scenario

	Indexes           []string
	SamtoolsBinary    string
	BismarkBinary     string
	Bowtie2Binary     string
	STARBinary        string
	STARSJDBOverhang  int
	IndexBuildThreads int
}

// BuildResult identifies the immutable release created by BuildRelease.
type BuildResult struct {
	RegistryRoot   string
	ReleaseRoot    string
	DefinitionPath string
	ManifestPath   string
	ChecksumsPath  string
	ManifestDigest string
}
