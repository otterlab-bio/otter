# 3. Quick start

Otter has two authoring tracks. They are marked on disk and refuse to mix:

| Track | Marker | Executor | Config artifact |
| --- | --- | --- | --- |
| Canonical v1 (default) | `project.lock.yaml` | `craftmake` | `project.yaml` + `references.lock.yaml` |
| Legacy compatibility (`--legacy`) | `.otter/assets.manifest.json` | `snakemake` | `config/otter.yaml` |

Start with the canonical track. Use the legacy track only for an existing project that was already created that way, or when you need the established Snakemake compatibility path.

## Canonical v1 track

### 1. Initialize the project

```bash
otter init my_project
```

This pins the packaged workflow assets into `my_project/` and records their digests:

```text
my_project/
├── project.lock.yaml   # track marker and pinned asset digests
├── workflows/          # packaged workflow assets
├── rules/              # Snakemake compatibility rules
├── environments/       # packaged environment declarations
├── schemas/            # packaged JSON schemas
└── runs/               # immutable run snapshots, written by config resolve
```

### 2. Provide a reference registry release

Canonical projects lock a logical reference id and release digest; they never copy genome files into the project. Build a release once, or select an existing one:

```bash
export OTTER_REFERENCE_ROOT=/shared/otter/references
otter reference build \
  --id hg38 --release GRCh38.p14 \
  --organism 'Homo sapiens' --assembly GRCh38 \
  --fasta /staging/genome.fa.gz --gtf /staging/genes.gtf.gz
```

See `reference registry` for the layout, manifest, and checksum contract.

### 3. Create the project intent

```bash
otter create \
  --output my_project \
  --fastq /data/fastq \
  --pdata /data/samples.csv \
  --mode RRBS \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --reference-primary hg38@GRCh38.p14
```

This verifies every declared reference against the registry and writes `project.yaml`, `samples.tsv`, and `references.lock.yaml`. `samples.tsv` records absolute FASTQ paths plus group and adapter columns; changing it later requires a new run.

For PDX scenarios, name the graft and host references instead:

```bash
otter create \
  --output my_project \
  --fastq /data/fastq \
  --mode RNASEQ \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --reference-graft hg38@GRCh38.p14 \
  --reference-host mm10@GRCm38.p6
```

### 4. Resolve an immutable run

```bash
otter config validate --config my_project/project.yaml --schema v1

otter config resolve \
  --project my_project/project.yaml \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --backend local
```

On a shared cluster, record the site once instead of repeating flags. `otter
site generate` probes the cluster and writes the profile; on a SLURM site it
needs the policy fields it cannot discover:

```bash
otter site generate --id production --backend slurm \
  --partition cpu112c --account genomics \
  --reference-root "$OTTER_REFERENCE_ROOT"
otter site validate production

otter config resolve --project my_project/project.yaml --site production --backend slurm
```

With `--site` and no `--reference-root`, the registry root comes from the profile.

Resolution prints the snapshot path. The snapshot is read-only and binds scenario, executor, backend, references, inputs, digests, and resources:

```text
my_project/runs/run-20260915T094657Z-eghubq/run.yaml
```

### 5. Dry-run, then run

```bash
otter run \
  --run-id run-20260915T094657Z-eghubq \
  --project-dir my_project \
  --executor craftmake \
  --phase step1 \
  --backend local \
  --dry-run \
  --foreground
```

`--dry-run` maps to `craftmake plan`: it resolves the workflow catalog and reports the task graph without executing anything. Drop `--dry-run` to execute, and swap `--backend local` for `--backend slurm` on a configured cluster.

`--run-id` is a convenience for `--config my_project/runs/<run-id>/run.yaml`. Otter never resolves `project.yaml` implicitly at run time; the snapshot must already exist.

## Legacy compatibility track

Both commands must agree on the track, so pass `--legacy` to both.

```bash
otter init my_project --legacy

otter create --legacy \
  --fastq /data/fastq \
  --mode RRBS \
  --pdata /data/samples.xlsx \
  --output my_project/userspace \
  --jobid demo_rrbs
```

The generated configuration is `my_project/userspace/demo_rrbs/config/otter.yaml`, and reference paths are validated with existence checks against `inst/` rather than against a registry.

```bash
otter assets verify --project my_project --strict
otter config validate --config my_project/userspace/demo_rrbs/config/otter.yaml --schema legacy

otter run \
  --config my_project/userspace/demo_rrbs/config/otter.yaml \
  --executor snakemake \
  --engine local \
  --dry-run \
  --foreground
```

Executing this track needs an `enva` or conda runtime providing Snakemake; `--dry-run` still validates configuration and workflow planning.

### Migrating a legacy project to v1

`otter config migrate` converts a legacy configuration into a canonical project. Every reference must be named as `id@release`, because the legacy configuration records filesystem paths rather than registry releases:

```bash
otter config migrate \
  --input my_project/userspace/demo_rrbs/config/otter.yaml \
  --output my_project/project.yaml \
  --reference-primary hg38@GRCh38.p14
```

Migration reports adopted fields, deprecated fields, and conflicts, and writes nothing when a conflict remains.

## Inspect and control tasks

```bash
otter task list
otter task status <task-id>
otter task logs <task-id> --follow
otter task stop <task-id>
otter status my_project
```

Without `--foreground`, Otter creates a background task and prints its task ID.

[Back to the manual](README.md) · [Next: analysis modes](04-analysis-modes.md)
