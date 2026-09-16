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

The legacy compatibility track requires `--legacy` on **both** authoring commands:

```bash
otter init my_project --legacy
otter create --legacy --fastq ./fastq --pdata ./samples.csv \
  --mode RRBS --output my_project/userspace --jobid demo_rrbs
otter run --config my_project/userspace/demo_rrbs/config/otter.yaml \
  --executor snakemake \
  --engine local --foreground
```

Legacy `otter.yaml` is an authoring format that no executor accepts directly;
migrate and resolve it first (see [chapter 8](08-reference-migration.md)).

## Before you begin

- Linux or macOS for local development; Linux with SLURM is the primary production target.
- Paired FASTQ files and a matching pdata file for `create`.
- A reference registry release for the selected scenario, addressed as
  `<id>@<release>` (see `reference registry`).
- A site profile when the machine is not the default target; generate one with
  `otter site generate` (see `site profiles`).
- `otter-snakemake` for the Snakemake compatibility path.
- `enva` and the managed runtime environments when using the release workflow setup.

[Back to documentation hub](../README.md)
