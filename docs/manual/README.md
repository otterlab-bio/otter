# Otter user manual

**OTTER: Orchestrated Transcriptomic, Tumor-xenograft (PDX/CDX), and Epigenomic Reporting Workflow**

This manual is organized around the first successful run rather than the internal package layout.

## Choose a path

| Goal | Chapter |
| --- | --- |
| Install a release or build from source | [1. Installation](01-installation.md) |
| Prepare FASTQ files and sample metadata | [2. Data preparation](02-data-preparation.md) |
| Create and run your first project | [3. Quick start](03-quickstart.md) |
| Select RRBS, WGBS, RNA-seq, or PDX behavior | [4. Analysis modes](04-analysis-modes.md) |
| Use custom references, SLURM resources, recovery, and canonical snapshots | [5. Advanced usage](05-advanced-usage.md) |
| Understand the companion tools | [6. Component reference](06-subtools.md) |
| Diagnose common failures | [7. FAQ](07-faq.md) |
| Migrate legacy reference genomes | [8. Reference migration](08-reference-migration.md) |

## The current execution model

```text
otter → craftmake → enva → operators → bamdriver
```

The production compatibility path remains Snakemake. Craftmake is the native Go executor under integration and validation. Use the executor explicitly when working with canonical immutable `otter.run/v1` snapshots; do not treat it as an undocumented fallback for every legacy project.

## Core command flow

The default path authors a canonical v1 project, which is what `otter run` accepts:

```bash
otter init my_project
otter create \
  --output my_project \
  --fastq ./fastq \
  --pdata ./samples.csv \
  --mode RRBS \
  --jobid demo_rrbs \
  --reference-root /shared/references \
  --reference-primary hg19@GRCh37.p13-gencode-v19
otter config validate --config my_project/project.yaml --schema v1
otter config resolve --project my_project/project.yaml --backend local
otter run --config my_project/runs/<run-id>/run.yaml \
  --executor craftmake --phase step1 --backend local --foreground
```

`otter create` writes a resolvable project directly: it locks the declared
references into `references.lock.yaml`, and `config resolve` writes the immutable
`run.yaml` that `otter run` consumes. A reference selection is always
`<id>@<release>`, for example `hg19@GRCh37.p13-gencode-v19`.

`otter build` chains init, create, validate, and resolve into one command and
stops before execution:

```bash
otter build \
  --project-root my_project \
  --fastq ./fastq \
  --pdata ./samples.csv \
  --mode RRBS \
  --reference-root /shared/references \
  --reference-primary hg19@GRCh37.p13-gencode-v19 \
  --backend local
```

It defaults to the `craftmake` executor; `--executor snakemake` records the
Snakemake compatibility executor in the snapshot instead.

The legacy compatibility track requires `--legacy` on **both** authoring commands:

```bash
otter init my_project --legacy
otter create --legacy --fastq ./fastq --pdata ./samples.csv \
  --mode RRBS --output my_project/userspace --jobid demo_rrbs
```

Legacy `otter.yaml` is an authoring format that no executor accepts directly, so
this track is authoring-only as written. To run it, migrate into a canonical root
and execute the resulting snapshot:

```bash
otter init migrated
otter config migrate \
  --input my_project/userspace/demo_rrbs/config/otter.yaml \
  --output migrated/project.yaml \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --reference-primary hg19@GRCh37.p13-gencode-v19
otter config resolve --project migrated/project.yaml \
  --reference-root "$OTTER_REFERENCE_ROOT" --backend local
otter run --config migrated/runs/<run-id>/run.yaml \
  --executor snakemake --dry-run --foreground
```

See [chapter 8](08-reference-migration.md) for the migration contract.

## Before you begin

- Linux or macOS for local development; Linux with SLURM is the primary production target.
- Paired FASTQ files and a matching pdata file for `create`.
- A reference registry release for the selected scenario, addressed as
  `<id>@<release>` (see [reference migration](08-reference-migration.md)).
- A site profile when the machine is not the default target; generate one with
  `otter site generate` (see [advanced usage](05-advanced-usage.md)).
- `otter-snakemake` for the Snakemake compatibility path.
- `enva` and the managed runtime environments when using the release workflow setup.

[Back to documentation hub](../README.md)
