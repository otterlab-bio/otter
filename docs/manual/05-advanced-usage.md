# 5. Advanced usage

## Custom references

On the canonical track, a reference is not a filesystem path — it is a **registry
release** addressed as `<id>@<release>`. To use your own genome, publish it to the
registry once, then select it by name:

```bash
otter reference build \
  --id myspecies --release ASM123-ensembl-110 \
  --organism 'Mus musculus' --assembly ASM123 \
  --fasta /refs/asm123.fa.gz --gtf /refs/asm123.gtf.gz

otter create \
  --output my_project \
  --fastq ./fastq --pdata ./samples.csv --mode RNASEQ \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --reference-primary myspecies@ASM123-ensembl-110
```

`create` verifies the release against the registry and locks its manifest digest,
so the project never records a path that can drift. For PDX, name the roles
separately with `--reference-graft` and `--reference-host`.

The legacy track instead takes literal paths, because it predates the registry:

```bash
otter create --legacy \
  --fastq ./fastq --mode RNASEQ --pdata ./samples.xlsx \
  --species1 hg38 \
  --genome1-fasta /refs/hg38.fa \
  --genome1-index /refs/hg38/bismark \
  --gtf1 /refs/hg38.gtf \
  --star-index1 /refs/hg38/star \
  --output my_project/userspace --jobid demo_rrbs
```

The `--species1`, `--genome1-*`, `--gtf1`, and `--star-index1` flags are
**legacy-track only**. The canonical track accepts them without complaint and
ignores them, so passing one alongside `--reference-primary` silently does
nothing — check `project.yaml` if you are unsure which selection was recorded.

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

## Resources and SLURM

The resource envelope is **not** a run-time flag. It is declared in `project.yaml`,
fixed into the snapshot when the run is resolved, and authoritative afterwards:

```yaml
resources:
  defaults:
    cores: 4
    memory: 8GiB
    time: 08:00:00
    partition: normal
  phases:
    step2:
      cores: 8
      memory: 16GiB
      partition: fat
```

`memory` takes `MiB` or `GiB`; `time` takes `HH:MM:SS`. `phases` overrides
`defaults` for a named phase.

Select the backend and the site at resolve time, and the envelope travels with the
snapshot:

```bash
otter site generate --id production --backend slurm \
  --partition normal --account genomics \
  --reference-root "$OTTER_REFERENCE_ROOT"

otter config resolve --project my_project/project.yaml \
  --site production --backend slurm
```

Resolution validates the envelope against the target — an unreachable partition or
a request larger than the machine is rejected there, not at submission time.

Confirm partition names and account/QOS limits with `sinfo` and your site
administrator before resolving.

## Dry-run, resume, and background control

Every run is executed from a snapshot, so validate one before committing resources:

```bash
otter run \
  --config my_project/runs/<run-id>/run.yaml \
  --executor craftmake \
  --phase step1 \
  --backend local \
  --dry-run \
  --foreground
```

`--dry-run` reports the resolved task graph and the resource envelope it validated
without executing anything.

`resume` is a recovery operation, not a way to change inputs or references in
place.

## Changing a run contract

The snapshot is immutable, so the run-time flags that would change its contract are
refused rather than silently applied:

```text
Error: --step1-cores cannot override an immutable run snapshot; resolve a new run.yaml instead
Error: --engine slurm conflicts with immutable run backend local
```

This includes `--backend`/`--engine`, every `--stepN-*` and `--slurm-*` resource
flag, and `--copy-fastq`. `--parallel-jobs` and `--load-ratio` remain settable,
because they size the local pool without altering what a task is.

To change the contract, resolve a new snapshot. Editing `project.yaml` also
invalidates existing runs: the snapshot pins the project digest, so a later
`otter run` reports `run snapshot drift detected` and refuses to execute a run
whose intent has moved.

```bash
otter config resolve --project project.yaml --backend slurm --site production
otter run --config runs/<new-run-id>/run.yaml \
  --executor craftmake --phase step1 --dry-run
```

## Artifact verification

Published run artifacts can be checked after execution:

```bash
otter artifact verify runs/<run-id>/run.yaml
```

The manifest binds artifacts to the immutable run identity and verifies declared checksums. Manifest completion is not a substitute for a scientific comparator.

[Back to the manual](README.md) · [Next: component reference](06-subtools.md)
