# 9. CLI reference

This chapter maps the public command tree to its owning contract. Use
`otter <command> --help` as the exact flag source for the installed release.

## Project and execution commands

| Command | Purpose | Important boundary |
| --- | --- | --- |
| `otter init <project>` | Pin workflow assets and schemas into a canonical project | Refuses existing canonical or legacy project markers. |
| `otter create` | Pair FASTQs, validate pdata/references, and write project intent | Canonical-only; PDX requires both graft and host reference roles. |
| `otter build` | Run init → create → validate → resolve | Stops before execution; prints the absolute snapshot path. |
| `otter config validate` | Validate `v1`, `legacy`, or auto-detected configuration | Legacy validation exists only to support migration. |
| `otter config migrate` | Convert an existing legacy `otter.yaml` into v1 intent | Requires reference release identities and a canonical root. |
| `otter config resolve` | Freeze intent, references, site/backend, and resources into `run.yaml` | The snapshot is immutable and authoritative at run time. |
| `otter run` | Plan or execute one resolved phase through Craftmake or Snakemake | Raw `project.yaml` and legacy `otter.yaml` are rejected. |
| `otter status [project]` | Summarize a project/run state | Read-only. |
| `otter task list/status/logs/report/stop` | Monitor and control background Otter tasks | `task report` exports controller/executor evidence; it is not a scientific comparator. |

## Site and asset commands

```bash
otter site list
otter site generate --id production --backend slurm \
  --partition cpu --account genomics \
  --reference-root /shared/otter/references
otter site validate production
```

`site generate` probes machine facts but requires explicit cluster policy.

```bash
otter assets stamp  --project old_project
otter assets verify --project old_project --strict
otter assets print  --project old_project
```

`assets` manages the compatibility-era `.otter/assets.manifest.json`; canonical
v1 projects use `project.lock.yaml` and resolved workflow digests.

## Reference commands

Build and atomically publish an immutable registry release:

```bash
otter reference build \
  --registry-root /shared/otter/references \
  --id hg38 --release GRCh38.p14 \
  --organism "Homo sapiens" --assembly GRCh38 \
  --fasta genome.fa.gz --gtf genes.gtf.gz \
  --indexes bismark,bowtie2,star \
  --index-build-threads 16
```

`samtools`, `bismark_genome_preparation`, `bowtie2-build`, and `STAR` are the
default tool names. In the release runtime they are provided by `otter-core`.

Promote a run's effective reference selection back to the project lock:

```bash
# Preview only.
otter reference promote runs/<run-id>/run.yaml

# Apply atomically and append the promotion audit record.
otter reference promote runs/<run-id>/run.yaml --confirm
```

## Artifact commands

```bash
# Build and immutably publish results/artifacts.json from declarations.
otter artifact publish runs/<run-id>/run.yaml artifact-declarations.json

# Verify the snapshot boundary, manifest, and checksums.
otter artifact verify runs/<run-id>/run.yaml

# Or verify an explicitly located manifest.
otter artifact verify runs/<run-id>/run.yaml \
  --manifest /absolute/results/artifacts.json

# Compare two already verified publications.
otter artifact compare \
  runs/<left-run-id>/run.yaml \
  runs/<right-run-id>/run.yaml
```

Manifest completion proves declared identity/integrity, not scientific parity.

## Acquisition provenance

Convert a verified Craftmake SRA decode manifest into immutable Otter
provenance:

```bash
otter acquisition publish \
  --decode-manifest decode-manifest.json \
  --output acquisition.json \
  --scenario rrbs \
  --primary-id hg38 \
  --primary-release GRCh38.p14 \
  --primary-manifest-sha256 <digest>
```

For `bs-pdx`/`rna-pdx`, replace the primary flags with complete graft and host
triples: `--graft-id`, `--graft-release`, `--graft-manifest-sha256`,
`--host-id`, `--host-release`, and `--host-manifest-sha256`. The command
verifies referenced files and refuses to overwrite the create-only output.

## Executor benchmark evidence

Collect matched, completed SLURM accounting cells:

```bash
otter benchmark collect \
  --left-run runs/craftmake/run.yaml \
  --left-jobs 123,124 \
  --right-run runs/snakemake/run.yaml \
  --right-jobs 200,201 \
  --phase step2 \
  --output executor-benchmark.json
```

Both snapshots must represent the same immutable comparison boundary. The
output is create-only evidence; it does not assert that one executor is
universally faster.

[Back to the manual](README.md)
