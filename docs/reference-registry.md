# Reference registry contract

The reference registry stores immutable genome releases outside project directories. Projects lock logical IDs and release digests; site resolution supplies the actual mount path when creating a run snapshot.

## Release naming is mandatory

A reference is always selected as `id@release`; there is no way to reference a
registry entry by path. The release label is semantic, not decorative — it pins
the assembly and the annotation provenance, which is the only reason a locked
reference is reproducible later.

The convention used in the deployed registry is
`<assembly>-<annotation-source>-<annotation-version>`:

```text
hg19@GRCh37.p13-gencode-v19
hg38@GRCh38-gencode-v44
mm10@GRCm38-gencode-M25
mm9@NCBIM37-gencode-M1
mm10-canary@GRCm38-ensembl-100-chr19
```

`reference.assembly` carries the assembly on its own (`GRCh37.p13`), while
`reference.release` carries assembly plus annotation provenance. `aliases` let a
short id resolve too, so `human@GRCh37.p13-gencode-v19` finds the same release as
`hg19@GRCh37.p13-gencode-v19`.

A corrected genome needs a **new release**, never an edit to an existing one.

## Directory layout

```text
$OTTER_REFERENCE_ROOT/
└── genomes/<reference-id>/<release>/
    ├── reference.yaml
    ├── manifest.json
    ├── checksums.sha256
    ├── fasta/
    ├── annotations/
    └── indexes/
        ├── bismark/
        ├── bowtie2/
        └── star/
```

A release directory is write-once. Corrections create a new release or manifest identity; an existing release is never overwritten in place.

## Build a release

The root CLI currently exposes `reference build` and publishes through a staging directory and atomic rename:

```bash
export OTTER_REFERENCE_ROOT=/shared/otter/references

otter reference build \
  --id hg38 \
  --release GRCh38.p14 \
  --organism 'Homo sapiens' \
  --assembly GRCh38 \
  --fasta /staging/genome.fa.gz \
  --gtf /staging/genes.gtf.gz \
  --indexes bismark,bowtie2,star
```

The command uses `samtools faidx` for the FASTA index and real index builders for selected assets. Placeholder index files are not valid registry releases. The default registry root is `$OTTER_REFERENCE_ROOT`, then `~/.otter/references` when the variable is unset.

The dedicated Gate 6 reference-build workflow is a separate operational path. Its source archives, provider checksums, compute-node build logs, resource policy, and accepted releases are documented in [Paracloud operations](gate6-paracloud-operations.md). Those historical releases are evidence; they are not implied by running the minimal command above.

## Reference metadata

Each release contains identity, assets, checksums, tools, and compatibility information:

```yaml
schema_version: otter.reference/v1
reference:
  id: hg38
  release: GRCh38.p14
  organism: Homo sapiens
  assembly: GRCh38
assets:
  fasta:
    path: fasta/genome.fa.gz
    sha256: sha256:...
  annotations:
    - id: gencode-v44
      type: gtf
      path: annotations/genes.gtf.gz
      sha256: sha256:...
  indexes:
    - type: bismark
      path: indexes/bismark
      reference_fasta_sha256: sha256:...
      tool: bismark
      tool_version: 0.24.2
compatibility:
  scenarios: [rrbs, wgbs, rnaseq, bs-pdx, rna-pdx]
```

An index declaration must identify the FASTA digest, builder/version, relevant parameters, output files, and supported scenarios. Directory indexes are expanded to file-level manifest entries.

## Project lock and resolution

`references.lock.yaml` records logical identity and manifest digest, not a machine-specific mount path:

```yaml
schema_version: otter.references.lock/v1
references:
  primary:
    id: hg38
    release: GRCh38.p14
    manifest_digest: sha256:...
```

Resolution verifies the lock, release metadata, manifest, checksums, FASTA/FAI relationship, index/reference relationship, scenario compatibility, and compute-node visibility before a production submission. The resolved absolute paths and digests are copied into `run.yaml`.

```text
selection
  → registry root from site
  → reference.yaml
  → manifest/checksum verification
  → scenario asset selection
  → absolute paths in run.yaml
  → scheduler preflight
```

Workflows must consume typed resolved assets from `run.yaml`; they must not reconstruct FASTA, GTF, or index paths from strings.

## Run override and promotion

A reference override belongs to one new run and does not change the project lock. Promotion is an explicit, audited action:

```bash
otter reference promote runs/<run-id>/run.yaml
otter reference promote runs/<run-id>/run.yaml --confirm
```

The first command previews the lock diff. The second verifies the resolved release, atomically updates `references.lock.yaml`, and appends the old/new lock and source run ID to `.otter/reference-promotions.jsonl`.

Resume cannot switch references. Re-resolve and create a new run when the effective reference changes.

## Dataset archives

Two Hugging Face dataset archives exist, and they have different jobs. Neither
substitutes for the other.

| Archive | Role | Contents |
|---|---|---|
| [`Genomiclab/xdxtools-genomes`](https://huggingface.co/datasets/Genomiclab/xdxtools-genomes) | **Legacy reference archive** | Pre-built Bismark and STAR tarballs per assembly, published before the registry contract existed |
| [`fallingstar10/xdxtools-genomes`](https://huggingface.co/datasets/fallingstar10/xdxtools-genomes) | **v1 contract archive** | Designated offline snapshot of registry releases, in the directory layout above |

The legacy archive is what the Snakemake compatibility track consumes: a legacy
project extracts its tarballs into that project's own `inst/` directory, and
`scripts/install.sh` downloads them. It predates the contract, so it carries no
`reference.yaml`, no `manifest.json`, no `checksums.sha256`, and no release label
— nothing about a file taken from it is verifiable. That gap is what the registry
contract exists to close, and it is why the legacy track is compatibility, not the
canonical authoring path. `otter init --legacy` points at this archive. It is
frozen as a historical archive and will not change.

The v1 contract archive is the designated home for offline registry-replay snapshots.
It is **not populated today and no population step is scheduled**: as of 2026-09-16 both
archives hold the same eight tarballs — `hg19`, `hg38`, `GRCm38`, `mm39` ×
{Bismark, STAR}, 8–31 GB each — and neither contains `reference.yaml`,
`manifest.json`, or `checksums.sha256`. CI keeps fetching FASTA and GTF from the
official providers and verifying them against the contract checksums.

Two properties to settle if that archive is ever populated:

- **Assembly coverage differs.** The archived assemblies are `hg19`, `hg38`,
  `GRCm38` (the `mm10` assembly), and `mm39`. The registry release set is `hg19`,
  `hg38`, `mm9`, `mm10`, and `mm10-canary`, so `mm9` is absent from the archive
  while `mm39` is not used by the registry.
- **Index tarballs are not a contract.** A contract release needs `reference.yaml`,
  the manifest, and per-release checksums alongside `fasta/`, `annotations/`, and
  `indexes/` in the layout above, not only an index archive.

A release taken from either archive is still subject to the naming and
sealed-directory rules in this document once it enters a registry.

These two are **genome** archives. Input fixtures are a separate dataset:
`fallingstar10/otter-data` holds the downsampled FASTQ and mixture fixtures the
Note 4 workflows consume, addressed by an immutable revision
(`xenofilx/benchmark/note4/README.md` records the mapping). Do not look for
reference releases there, and do not look for fixtures here.

## Gate 6 boundary

The accepted Gate 6 reference evidence covers controlled acquisition, provider checksum verification, immutable publication, and compute-node visibility for the documented release set. The `mm10-canary` is a small technical reference for scheduler/index/publication checks; it is not a full-genome `mm10` or `mm38` production reference.

[Back to the documentation hub](README.md)
