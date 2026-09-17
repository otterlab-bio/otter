# 8. Reference Genome Migration Guide

This chapter provides a guide and reference script for migrating legacy reference genome files (previously organized under `$beaverhome/inst/`) into the modern, immutable Otter Reference Registry (`$OTTER_REFERENCE_ROOT`).

---

## 1. Background & Architecture Comparison

In legacy Beaverflow/xdxtools deployments, reference genomes were stored in an ad-hoc directory structure inside each installation:

```text
# Legacy layout: $beaverhome/inst/
$beaverhome/inst/
├── hg19/
│   ├── hg19_cpgIsland.bed
│   ├── hg19_CpG_sites.gz
│   └── hg19_CpG_sites.gz.tbi
├── pdx/
│   ├── homo_sapiens/
│   │   ├── hg19.fasta
│   │   ├── Bisulfite_Genome/
│   │   └── chrs.txt
│   └── mouse/
│       ├── GRCm38.fasta
│       ├── GRCm38.fasta.fai
│       └── Bisulfite_Genome/
└── rnaseq/
    └── homo_sapiens/
        ├── hg19.fasta
        ├── hg19.ensGene.gtf
        └── (STAR indexes: Genome, SA, SAindex, *.tab, *.txt)
```

**Limitations of the legacy structure**:
- References were tied to local project paths or user home directories via hardcoded relative paths like `../../inst/pdx/homo_sapiens`.
- No cryptographic manifests (`manifest.json`) or file checksums (`checksums.sha256`) existed to ensure reproducibility or detect bitrot.
- Files were mutable and subject to accidental modification or overwriting.

---

## 2. The Modern Reference Registry Contract

In Otter, all reference genomes are managed centrally in an immutable registry outside project spaces:

```text
$OTTER_REFERENCE_ROOT/
└── genomes/<reference-id>/<release>/
    ├── reference.yaml          # Metadata (schema: otter.reference/v1)
    ├── manifest.json           # SHA-256 digests of every file in the release
    ├── checksums.sha256        # Standard sha256sum verification manifest
    ├── fasta/
    │   ├── <reference-id>.fa
    │   └── <reference-id>.fa.fai
    ├── annotations/
    │   └── <reference-id>.gtf
    └── indexes/
        ├── bismark/            # Bismark CT/GA converted genome
        ├── bowtie2/            # Bowtie2 index files
        └── star/               # STAR index files
```

**Key properties**:
- **Write-once & Sealed**: Published reference releases are atomically renamed and sealed read-only (`chmod 0555` / `0444`).
- **Cryptographically Auditable**: Every file has its SHA-256 digest tracked in `manifest.json`.
- **Decoupled from Projects**: Projects record only the logical identity and manifest digest in `references.lock.yaml`:
  ```yaml
  schema_version: otter.references.lock/v1
  references:
    primary:
      id: hg19
      release: GRCh37.p13-gencode-v19
      manifest_digest: sha256:4f8e...
  ```

  The key is the reference **role** (`primary`, `secondary`, `graft`, `host`), not the
  reference id, and the digest field is `manifest_digest`. A release label is
  mandatory and named `<assembly>-<annotation-source>-<annotation-version>`; the
  layout and digest contract are defined in section 2 above.

---

## 3. Migration Tool: `migrate_legacy_reference.sh`

The repository provides an automated migration script: `scripts/migrate_legacy_reference.sh`.

### CLI Options

| Flag | Default | Description |
| --- | --- | --- |
| `--legacy-inst PATH` | `$beaverhome/inst` | Path to legacy `inst/` directory |
| `--species NAME` | `all` | Species to migrate: `hg19`, `mm10`, or `all` |
| `--registry-root PATH` | `$OTTER_REFERENCE_ROOT` or `~/.otter/references` | Destination Reference Registry root |
| `--mode MODE` | `rebuild` | Migration mode: `ingest` (fast, hardlinks existing indexes) or `rebuild` (build from source) |
| `--threads NUM` | `4` | Number of CPU threads for index building in `rebuild` mode |
| `--otter-core-bin PATH` | auto-detected | Path to `otter-core` environment `bin/` |
| `--otter-bin PATH` | auto-detected | Path to `otter` executable |
| `--dry-run` | `false` | Preview discovered paths and planned commands |

---

## 4. Migration Execution

### Step 1: Environment Check

Ensure `otter` and `enva` are in your `PATH`, and `otter-core` is created:

```bash
which otter enva craftmake
enva run otter-core -- bismark --version
enva run otter-core -- bowtie2 --version
```

> **Site note (Dedicated login/submit nodes)**: On clusters where compute or Slurm submission tools are only available from a designated gateway or submit node (for example, switching from `gangliamaster` to `login1` on the Hangzhou cluster via `ssh login1`), run the commands from that designated node.

### Step 2: Run Dry-Run Preview

```bash
bash scripts/migrate_legacy_reference.sh \
  --legacy-inst "$beaverhome/inst" \
  --registry-root "$OTTER_REFERENCE_ROOT" \
  --species hg19 \
  --dry-run
```

Expected output:
```text
  [INFO]  Legacy inst root: /shared/beaverflow/inst
  [INFO]  Target registry : /shared/otter/references
  [INFO]  Source FASTA: .../inst/pdx/homo_sapiens/hg19.fasta (3.0G)
  [INFO]  Source GTF  : .../inst/rnaseq/homo_sapiens/hg19.ensGene.gtf (448M)
  [INFO]  Target indexes: bismark,bowtie2,star
  ✓ Migration procedure completed.
```

### Step 3: Execute Migration

**Option A — Ingest existing indexes (recommended)**:
When the legacy directory already contains pre-built Bismark or STAR indexes, `--mode ingest` reuses them via hardlinks (zero additional disk space on the same filesystem) and generates the compliant `reference.yaml`, `manifest.json`, and `checksums.sha256` in under two minutes:

```bash
bash scripts/migrate_legacy_reference.sh \
  --legacy-inst "$beaverhome/inst" \
  --registry-root "$OTTER_REFERENCE_ROOT" \
  --species hg19 \
  --mode ingest
```

**Option B — Rebuild indexes from source**:
Rebuilds all indexes from the source FASTA and GTF using `otter reference build`:

```bash
bash scripts/migrate_legacy_reference.sh \
  --legacy-inst "$beaverhome/inst" \
  --registry-root "$OTTER_REFERENCE_ROOT" \
  --species hg19 \
  --mode rebuild \
  --threads 16
```

> **Site note (HPC Slurm batch submission)**: When rebuilding a large reference genome on an HPC cluster, submit the migration as a batch job so intensive indexing runs on an allocated compute node rather than a shared head node:
> ```bash
> sbatch -p <partition> -c 16 --mem=64G -J migrate_ref \
>   --wrap="bash scripts/migrate_legacy_reference.sh \
>     --legacy-inst '$beaverhome/inst' \
>     --registry-root '$OTTER_REFERENCE_ROOT' \
>     --species hg19 \
>     --mode rebuild \
>     --threads 16"
> ```
> *(For example, on the Hangzhou cluster this used `-p cpu112c` submitted from `login1`)*.

---

## 5. Verifying the Migrated Reference Release

After migration completes, verify the release with `otter reference`:

```bash
export OTTER_REFERENCE_ROOT=/shared/otter/references

# Verify directory permissions (sealed read-only)
ls -ld $OTTER_REFERENCE_ROOT/genomes/hg19/GRCh37.p13-gencode-v19
# Expect permissions: dr-xr-xr-x

# Verify manifest and checksums
cd $OTTER_REFERENCE_ROOT/genomes/hg19/GRCh37.p13-gencode-v19
sha256sum -c checksums.sha256
```

---

## 6. Project Locking

In your analysis project directory, record the reference release in `references.lock.yaml`:

```yaml
schema_version: otter.references.lock/v1
references:
  primary:
    id: hg19
    release: GRCh37.p13-gencode-v19
    manifest_digest: sha256:<sha256-of-manifest.json>
```

When `otter config resolve` runs, it locates `$OTTER_REFERENCE_ROOT/genomes/hg19/GRCh37.p13-gencode-v19`, validates the manifest digest matches the lock, and embeds the verified path into the immutable `run.yaml`.

In practice `otter create --reference-primary hg19@GRCh37.p13-gencode-v19 --reference-root "$OTTER_REFERENCE_ROOT"` writes this lock for you.

---

[Back to the user manual](README.md) · [Next: FAQ](07-faq.md)
