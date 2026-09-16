# 5. Advanced usage

## Custom references

Override the default references when your site maintains its own FASTA, Bismark index, GTF, or STAR index:

```bash
otter create \
  --fastq ./fastq \
  --mode RNASEQ \
  --pdata ./samples.xlsx \
  --species1 hg38 \
  --genome1-fasta /refs/hg38.fa \
  --genome1-index /refs/hg38/bismark \
  --gtf1 /refs/hg38.gtf \
  --star-index1 /refs/hg38/star
```

For PDX, provide the corresponding `genome2-*`, `gtf2`, and `star-index2` values. The command validates custom paths before writing the project.

## FASTQ suffixes

For non-standard names, set both suffixes explicitly:

```bash
otter create \
  --fastq ./raw_fastq \
  --mode WGBS \
  --suffix1 _1.fq.gz \
  --suffix2 _2.fq.gz \
  --pdata ./samples.xlsx
```

## SLURM resources

Use global settings for a simple run:

```bash
otter run \
  --config my_project/userspace/<jobid>/config/otter.yaml \
  --executor snakemake \
  --engine slurm \
  --slurm-partition normal \
  --slurm-cores 16 \
  --slurm-memory 64G \
  --parallel-jobs 8
```

Use per-step settings when a phase has a different resource envelope:

```bash
otter run \
  --config my_project/userspace/<jobid>/config/otter.yaml \
  --executor snakemake \
  --engine slurm \
  --step1-cores 8 --step1-memory 32G \
  --step2-cores 24 --step2-memory 128G --step2-partition fat \
  --step3-cores 8 --step3-memory 16G
```

`--load-ratio` controls the dynamic local/SLURM job pool. Confirm partition names and account/QOS limits with `sinfo` and your site administrator.

## Dry-run, resume, and background control

For an established legacy-compatible project:

```bash
otter run \
  --config my_project/userspace/<jobid>/config/otter.yaml \
  --executor snakemake \
  --engine local \
  --dry-run \
  --foreground
```

For a canonical v1 run, resolve an immutable snapshot first and execute it with Craftmake:

```bash
otter config resolve --project project.yaml --backend local
otter run \
  --config runs/<run-id>/run.yaml \
  --executor craftmake \
  --phase step1 \
  --backend local \
  --foreground
```

`resume` is a recovery operation, not a way to change inputs or references in place.

## Canonical immutable runs

The v1 path resolves a project into a run directory:

```bash
otter config migrate --input otter.yaml --output project.yaml
otter config resolve --project project.yaml --backend slurm
otter run --config runs/<run-id>/run.yaml \
  --executor craftmake --phase step1 --backend slurm
```

Craftmake rejects runtime flags that would change immutable resources, references, or input placement. Resolve a new snapshot when the run contract must change.

## Artifact verification

Published run artifacts can be checked after execution:

```bash
otter artifact verify runs/<run-id>/run.yaml
```

The manifest binds artifacts to the immutable run identity and verifies declared checksums. Manifest completion is not a substitute for a scientific comparator.

[Back to the manual](README.md) · [Next: component reference](06-subtools.md)
