package v1

const (
	ProjectSchemaVersion              = "otter.project/v1"
	RunSchemaVersion                  = "otter.run/v1"
	ReferenceSchemaVersion            = "otter.reference/v1"
	ReferencesLockSchemaVersion       = "otter.references.lock/v1"
	SRAAcquisitionSchemaVersion       = "otter.sra-acquisition/v1"
	ParityPolicyExecutorPhaseEnvelope = "executor-phase-envelope/v1"
)

type Scenario string

const (
	ScenarioRRBS   Scenario = "rrbs"
	ScenarioWGBS   Scenario = "wgbs"
	ScenarioRNASeq Scenario = "rnaseq"
	ScenarioBSPDX  Scenario = "bs-pdx"
	ScenarioRNAPDX Scenario = "rna-pdx"
)

type Toolchain string

const (
	ToolchainModern           Toolchain = "modern"
	ToolchainLegacyEquivalent Toolchain = "legacy-equivalent"
)

type Executor string

const (
	ExecutorCraftmake Executor = "craftmake"
	ExecutorSnakemake Executor = "snakemake"
)

type Backend string

const (
	BackendAuto  Backend = "auto"
	BackendLocal Backend = "local"
	BackendSlurm Backend = "slurm"
)

type ValueSource string

const (
	SourceDefault   ValueSource = "default"
	SourceProject   ValueSource = "project"
	SourceCLI       ValueSource = "cli"
	SourceProfile   ValueSource = "profile"
	SourceDetection ValueSource = "detection"
)

type ReferenceRole string

const (
	ReferenceRolePrimary   ReferenceRole = "primary"
	ReferenceRoleSecondary ReferenceRole = "secondary"
	ReferenceRoleGraft     ReferenceRole = "graft"
	ReferenceRoleHost      ReferenceRole = "host"
)

type OverrideSource string

const (
	OverrideSourceNone         OverrideSource = "none"
	OverrideSourceProject      OverrideSource = "project"
	OverrideSourceCLI          OverrideSource = "cli"
	OverrideSourceOverrideFile OverrideSource = "override-file"
)

type ProjectConfig struct {
	SchemaVersion string              `yaml:"schema_version" json:"schema_version"`
	Project       ProjectMetadata     `yaml:"project" json:"project"`
	Workflow      ProjectWorkflow     `yaml:"workflow" json:"workflow"`
	Execution     ProjectExecution    `yaml:"execution" json:"execution"`
	Samples       SamplesDeclaration  `yaml:"samples" json:"samples"`
	References    ProjectReferences   `yaml:"references" json:"references"`
	Resources     ProjectResources    `yaml:"resources,omitempty" json:"resources,omitempty"`
	Observability ObservabilityConfig `yaml:"observability,omitempty" json:"observability,omitempty"`
}

type ProjectMetadata struct {
	ID          string `yaml:"id" json:"id"`
	Description string `yaml:"description,omitempty" json:"description,omitempty"`
}

type ProjectWorkflow struct {
	Scenario         Scenario  `yaml:"scenario" json:"scenario"`
	Toolchain        Toolchain `yaml:"toolchain,omitempty" json:"toolchain,omitempty"`
	LegacyExtensions []string  `yaml:"legacy_extensions,omitempty" json:"legacy_extensions,omitempty"`
}

type ProjectExecution struct {
	Executor Executor `yaml:"executor,omitempty" json:"executor,omitempty"`
	Backend  Backend  `yaml:"backend,omitempty" json:"backend,omitempty"`
	Site     string   `yaml:"site,omitempty" json:"site,omitempty"`
}

type ProjectFieldSources struct {
	Toolchain ValueSource
	Executor  ValueSource
	Backend   ValueSource
	Site      ValueSource
}

type SamplesDeclaration struct {
	Manifest string `yaml:"manifest" json:"manifest"`
}

type ReferenceSelection string

type ProjectReferences struct {
	Primary   ReferenceSelection `yaml:"primary,omitempty" json:"primary,omitempty"`
	Secondary ReferenceSelection `yaml:"secondary,omitempty" json:"secondary,omitempty"`
	Species   []SpeciesReference `yaml:"species,omitempty" json:"species,omitempty"`
}

type SpeciesReference struct {
	Role      ReferenceRole      `yaml:"role" json:"role"`
	Selection ReferenceSelection `yaml:"selection" json:"selection"`
}

type ResourceSpec struct {
	Cores     int    `yaml:"cores,omitempty" json:"cores,omitempty"`
	Memory    string `yaml:"memory,omitempty" json:"memory,omitempty"`
	Time      string `yaml:"time,omitempty" json:"time,omitempty"`
	Partition string `yaml:"partition,omitempty" json:"partition,omitempty"`
}

type ProjectResources struct {
	Defaults ResourceSpec            `yaml:"defaults,omitempty" json:"defaults,omitempty"`
	Phases   map[string]ResourceSpec `yaml:"phases,omitempty" json:"phases,omitempty"`
}

type ObservabilityConfig struct {
	Metrics    bool `yaml:"metrics" json:"metrics"`
	RetainLogs bool `yaml:"retain_logs" json:"retain_logs"`
}

type SampleRecord struct {
	ID        string `yaml:"id" json:"id"`
	R1        string `yaml:"r1" json:"r1"`
	R2        string `yaml:"r2" json:"r2"`
	Group     string `yaml:"group,omitempty" json:"group,omitempty"`
	Batch     string `yaml:"batch,omitempty" json:"batch,omitempty"`
	AdapterR1 string `yaml:"adapter_r1,omitempty" json:"adapter_r1,omitempty"`
	AdapterR2 string `yaml:"adapter_r2,omitempty" json:"adapter_r2,omitempty"`
	R1SHA256  string `yaml:"r1_sha256,omitempty" json:"r1_sha256,omitempty"`
	R1Size    int64  `yaml:"r1_size_bytes,omitempty" json:"r1_size_bytes,omitempty"`
	R2SHA256  string `yaml:"r2_sha256,omitempty" json:"r2_sha256,omitempty"`
	R2Size    int64  `yaml:"r2_size_bytes,omitempty" json:"r2_size_bytes,omitempty"`
}

type SRAAcquisitionManifest struct {
	SchemaVersion string                 `json:"schema_version"`
	Acquisition   SRAAcquisitionIdentity `json:"acquisition"`
	Entries       []SRAAcquisitionEntry  `json:"entries"`
}

type SRAAcquisitionIdentity struct {
	ID        string    `json:"id"`
	CreatedAt Timestamp `json:"created_at"`
	Immutable bool      `json:"immutable"`
	Root      string    `json:"root"`
}

type SRAAcquisitionEntry struct {
	Scenario  Scenario                `json:"scenario"`
	Accession string                  `json:"accession"`
	Download  SRAArchive              `json:"download"`
	Output    SRAAcquiredFASTQPair    `json:"output"`
	Reference SRAAcquisitionReference `json:"reference"`
	Tool      SRAAcquisitionTool      `json:"tool"`
}

type SRAArchive struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
	MD5    string `json:"md5"`
}

type SRAAcquiredFASTQPair struct {
	R1 SRAAcquiredFASTQ `json:"r1"`
	R2 SRAAcquiredFASTQ `json:"r2"`
}

type SRAAcquiredFASTQ struct {
	Path              string `json:"path"`
	SHA256            string `json:"sha256"`
	Bytes             int64  `json:"bytes"`
	PairedRecordCount int64  `json:"paired_record_count"`
}

type SRAAcquisitionReference struct {
	Primary *SRAReferenceSelection  `json:"primary,omitempty"`
	Species []SRAReferenceSelection `json:"species,omitempty"`
}

type SRAReferenceSelection struct {
	Role           ReferenceRole `json:"role"`
	ID             string        `json:"id"`
	Release        string        `json:"release"`
	ManifestSHA256 string        `json:"manifest_sha256"`
}

type SRAAcquisitionTool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Command string `json:"command"`
}

type ReferencesLock struct {
	SchemaVersion     string                     `yaml:"schema_version" json:"schema_version"`
	References        map[string]LockedReference `yaml:"references" json:"references"`
	PromotedFromRunID string                     `yaml:"promoted_from_run_id,omitempty" json:"promoted_from_run_id,omitempty"`
}

type LockedReference struct {
	ID             string `yaml:"id" json:"id"`
	Release        string `yaml:"release" json:"release"`
	ManifestDigest string `yaml:"manifest_digest" json:"manifest_digest"`
}

type ReferenceDefinition struct {
	SchemaVersion string                 `yaml:"schema_version" json:"schema_version"`
	Reference     ReferenceIdentity      `yaml:"reference" json:"reference"`
	Assets        ReferenceAssets        `yaml:"assets" json:"assets"`
	Compatibility ReferenceCompatibility `yaml:"compatibility" json:"compatibility"`
}

type ReferenceIdentity struct {
	ID       string   `yaml:"id" json:"id"`
	Release  string   `yaml:"release" json:"release"`
	Organism string   `yaml:"organism" json:"organism"`
	Assembly string   `yaml:"assembly" json:"assembly"`
	Aliases  []string `yaml:"aliases,omitempty" json:"aliases,omitempty"`
}

type ReferenceAssets struct {
	Fasta       ReferenceFasta        `yaml:"fasta" json:"fasta"`
	Annotations []ReferenceAnnotation `yaml:"annotations,omitempty" json:"annotations,omitempty"`
	Indexes     []ReferenceIndex      `yaml:"indexes" json:"indexes"`
}

type ReferenceFasta struct {
	Path      string `yaml:"path" json:"path"`
	SHA256    string `yaml:"sha256" json:"sha256"`
	SizeBytes int64  `yaml:"size_bytes" json:"size_bytes"`
	FAI       string `yaml:"fai" json:"fai"`
}

type ReferenceAnnotation struct {
	ID     string `yaml:"id" json:"id"`
	Type   string `yaml:"type" json:"type"`
	Path   string `yaml:"path" json:"path"`
	SHA256 string `yaml:"sha256" json:"sha256"`
}

type ReferenceIndex struct {
	Type                 string               `yaml:"type" json:"type"`
	Path                 string               `yaml:"path" json:"path"`
	ReferenceFastaSHA256 string               `yaml:"reference_fasta_sha256" json:"reference_fasta_sha256"`
	Tool                 string               `yaml:"tool" json:"tool"`
	ToolVersion          string               `yaml:"tool_version" json:"tool_version"`
	Parameters           IndexBuildParameters `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	ManifestSHA256       string               `yaml:"manifest_sha256" json:"manifest_sha256"`
}

type IndexBuildParameters struct {
	SJDBOverhang int      `yaml:"sjdbOverhang,omitempty" json:"sjdbOverhang,omitempty"`
	Arguments    []string `yaml:"arguments,omitempty" json:"arguments,omitempty"`
}

type ReferenceCompatibility struct {
	Scenarios []Scenario `yaml:"scenarios" json:"scenarios"`
	Workflows []string   `yaml:"workflows,omitempty" json:"workflows,omitempty"`
}

type RunSnapshot struct {
	SchemaVersion string              `yaml:"schema_version" json:"schema_version"`
	Run           RunMetadata         `yaml:"run" json:"run"`
	Project       ResolvedProject     `yaml:"project" json:"project"`
	Workflow      ResolvedWorkflow    `yaml:"workflow" json:"workflow"`
	Execution     ResolvedExecution   `yaml:"execution" json:"execution"`
	Samples       []SampleRecord      `yaml:"samples" json:"samples"`
	References    ResolvedReferences  `yaml:"references" json:"references"`
	Paths         RunPaths            `yaml:"paths" json:"paths"`
	Digests       RunDigests          `yaml:"digests" json:"digests"`
	Observability ObservabilityConfig `yaml:"observability" json:"observability"`
	Parity        ParityConfig        `yaml:"parity" json:"parity"`
}

type Timestamp string

type RunMetadata struct {
	ID          string    `yaml:"id" json:"id"`
	CreatedAt   Timestamp `yaml:"created_at" json:"created_at"`
	Immutable   bool      `yaml:"immutable" json:"immutable"`
	ParentRunID string    `yaml:"parent_run_id,omitempty" json:"parent_run_id,omitempty"`
}

type ResolvedProject struct {
	ID   string `yaml:"id" json:"id"`
	Root string `yaml:"root" json:"root"`
}

type ResolvedWorkflow struct {
	Scenario         Scenario  `yaml:"scenario" json:"scenario"`
	Toolchain        Toolchain `yaml:"toolchain" json:"toolchain"`
	LegacyExtensions []string  `yaml:"legacy_extensions" json:"legacy_extensions"`
	AssetRoot        string    `yaml:"asset_root" json:"asset_root"`
}

type ResolvedExecutor struct {
	Value  Executor    `yaml:"value" json:"value"`
	Source ValueSource `yaml:"source" json:"source"`
}

type ResolvedBackend struct {
	Value    Backend         `yaml:"value" json:"value"`
	Source   ValueSource     `yaml:"source" json:"source"`
	Evidence BackendEvidence `yaml:"evidence" json:"evidence"`
}

type BackendEvidence struct {
	Cluster  string   `yaml:"cluster,omitempty" json:"cluster,omitempty"`
	Commands []string `yaml:"commands,omitempty" json:"commands,omitempty"`
	Reason   string   `yaml:"reason,omitempty" json:"reason,omitempty"`
}

type ResolvedString struct {
	Value  string      `yaml:"value" json:"value"`
	Source ValueSource `yaml:"source" json:"source"`
}

type ResolvedInt struct {
	Value  int         `yaml:"value" json:"value"`
	Source ValueSource `yaml:"source" json:"source"`
}

type ResolvedSlurmResources struct {
	Partition   ResolvedString `yaml:"partition" json:"partition"`
	Account     ResolvedString `yaml:"account" json:"account"`
	QOS         ResolvedString `yaml:"qos" json:"qos"`
	MaxJobs     ResolvedInt    `yaml:"max_jobs" json:"max_jobs"`
	DefaultTime ResolvedString `yaml:"default_time" json:"default_time"`
	ScratchRoot ResolvedString `yaml:"scratch_root" json:"scratch_root"`
}

type ResolvedExecution struct {
	Executor  ResolvedExecutor       `yaml:"executor" json:"executor"`
	Backend   ResolvedBackend        `yaml:"backend" json:"backend"`
	Site      ResolvedString         `yaml:"site" json:"site"`
	Resources ProjectResources       `yaml:"resources" json:"resources"`
	Slurm     ResolvedSlurmResources `yaml:"slurm,omitempty" json:"slurm,omitempty"`
}

type ReferenceSelections struct {
	Primary   ReferenceSelection `yaml:"primary,omitempty" json:"primary,omitempty"`
	Secondary ReferenceSelection `yaml:"secondary,omitempty" json:"secondary,omitempty"`
	Graft     ReferenceSelection `yaml:"graft,omitempty" json:"graft,omitempty"`
	Host      ReferenceSelection `yaml:"host,omitempty" json:"host,omitempty"`
}

type ResolvedReferences struct {
	ProjectSelection   ReferenceSelections `yaml:"project_selection" json:"project_selection"`
	EffectiveSelection ReferenceSelections `yaml:"effective_selection" json:"effective_selection"`
	Override           bool                `yaml:"override" json:"override"`
	OverrideSource     OverrideSource      `yaml:"override_source" json:"override_source"`
	Resolved           []ResolvedReference `yaml:"resolved" json:"resolved"`
}

type ResolvedReference struct {
	Role           ReferenceRole   `yaml:"role" json:"role"`
	ID             string          `yaml:"id" json:"id"`
	Release        string          `yaml:"release" json:"release"`
	Organism       string          `yaml:"organism,omitempty" json:"organism,omitempty"`
	RegistryRoot   string          `yaml:"registry_root" json:"registry_root"`
	ManifestDigest string          `yaml:"manifest_digest" json:"manifest_digest"`
	Fasta          ResolvedAsset   `yaml:"fasta" json:"fasta"`
	Annotations    []ResolvedAsset `yaml:"annotations" json:"annotations"`
	Indexes        []ResolvedAsset `yaml:"indexes" json:"indexes"`
}

type ResolvedAsset struct {
	Type   string `yaml:"type" json:"type"`
	Path   string `yaml:"path" json:"path"`
	SHA256 string `yaml:"sha256" json:"sha256"`
}

type RunPaths struct {
	RunRoot         string   `yaml:"run_root" json:"run_root"`
	Work            string   `yaml:"work" json:"work"`
	Results         string   `yaml:"results" json:"results"`
	Logs            string   `yaml:"logs" json:"logs"`
	State           string   `yaml:"state" json:"state"`
	Metrics         string   `yaml:"metrics" json:"metrics"`
	ProjectConfig   string   `yaml:"project_config,omitempty" json:"project_config,omitempty"`
	SamplesManifest string   `yaml:"samples_manifest,omitempty" json:"samples_manifest,omitempty"`
	ReferencesLock  string   `yaml:"references_lock,omitempty" json:"references_lock,omitempty"`
	WorkflowAssets  []string `yaml:"workflow_assets,omitempty" json:"workflow_assets,omitempty"`
}

type RunDigests struct {
	Project        string `yaml:"project" json:"project"`
	Samples        string `yaml:"samples" json:"samples"`
	WorkflowAssets string `yaml:"workflow_assets" json:"workflow_assets"`
	References     string `yaml:"references,omitempty" json:"references,omitempty"`
}

type ParityConfig struct {
	Policy string `yaml:"policy,omitempty" json:"policy,omitempty"`
}
