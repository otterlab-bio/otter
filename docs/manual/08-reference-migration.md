# 8. Reference Genome Migration Guide

This chapter describes the supported compatibility procedure for moving an
existing reference tree into the immutable Otter Reference Registry
(`$OTTER_REFERENCE_ROOT`). The source layout is deployment-specific: supply its
root explicitly with `--legacy-inst`, rather than relying on a historical
installation path or directory convention.

---

## 1. The Reference Registry Contract

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
  mandatory; the naming convention `<assembly>-<annotation-source>-<annotation-version>`
  is recommended but not enforced (the schema only requires `^[A-Za-z0-9._-]+$`).
  The layout and digest contract are defined in section 2 above.

---

## 2. Migration Tool: `migrate_legacy_reference.sh`

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

## 3. Migration Execution

### Step 1: Environment Check

Ensure `otter` and `enva` are in your `PATH`, and `otter-core` is created:

```bash
which otter enva craftmake
enva run otter-core -- bismark --version
enva run otter-core -- bowtie2 --version
```

### Step 2: Run Dry-Run Preview

```bash
export LEGACY_INST=/path/to/legacy/inst
export OTTER_REFERENCE_ROOT=/path/to/otter/references

bash scripts/migrate_legacy_reference.sh \
  --legacy-inst "$LEGACY_INST" \
  --registry-root "$OTTER_REFERENCE_ROOT" \
  --species hg19 \
  --dry-run
```

Expected output:
```text
  [INFO]  Legacy inst root: /path/to/legacy/inst
  [INFO]  Target registry : /path/to/otter/references
  [INFO]  Source FASTA: <discovered source FASTA>
  [INFO]  Source GTF  : <discovered source GTF>
  [INFO]  Target indexes: bismark,bowtie2,star
  ✓ Migration procedure completed.
```

### Step 3: Execute Migration

**Option A — Ingest existing indexes (recommended)**:
When the legacy directory already contains pre-built Bismark or STAR indexes, `--mode ingest` reuses them via hardlinks (zero additional disk space on the same filesystem) and generates the compliant `reference.yaml`, `manifest.json`, and `checksums.sha256` in under two minutes:

```bash
bash scripts/migrate_legacy_reference.sh \
  --legacy-inst "$LEGACY_INST" \
  --registry-root "$OTTER_REFERENCE_ROOT" \
  --species hg19 \
  --mode ingest
```

**Option B — Rebuild indexes from source**:
Rebuilds all indexes from the source FASTA and GTF using `otter reference build`:

```bash
bash scripts/migrate_legacy_reference.sh \
  --legacy-inst "$LEGACY_INST" \
  --registry-root "$OTTER_REFERENCE_ROOT" \
  --species hg19 \
  --mode rebuild \
  --threads 16
```

---

## 4. Verifying the Migrated Reference Release

After migration completes, verify the release with `otter reference`:

```bash
export OTTER_REFERENCE_ROOT=/path/to/otter/references

# Verify directory permissions (sealed read-only)
ls -ld $OTTER_REFERENCE_ROOT/genomes/hg19/GRCh37.p13-gencode-v19
# Expect permissions: dr-xr-xr-x

# Verify manifest and checksums
cd $OTTER_REFERENCE_ROOT/genomes/hg19/GRCh37.p13-gencode-v19
sha256sum -c checksums.sha256
```

---

## 5. Project Locking

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
