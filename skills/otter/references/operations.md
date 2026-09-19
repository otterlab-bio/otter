# otter operational runbook

Use this runbook for user-facing operation. Use `repo-map.md` and
`build-and-test.md` for code changes.

## 1. Verify an installation

```bash
command -v otter enva craftmake
otter --version
enva list --detailed
enva validate --all
enva run otter-core -- bismark --version
enva run otter-core -- bowtie2 --version
```

Bismark 3.1.0 and Bowtie2 2.5.5 belong to `otter-core`; do not instruct users
to install Bismark into a user-global Cargo directory.

## 2. Author a canonical project

Every project names an immutable reference release. For one-species work:

```bash
otter init my_project
otter create \
  --output my_project \
  --fastq /data/fastq \
  --pdata /data/samples.csv \
  --mode RRBS \
  --reference-root /shared/otter/references \
  --reference-primary hg38@GRCh38.p14

otter config validate --config my_project/project.yaml --schema v1
otter config resolve \
  --project my_project/project.yaml \
  --reference-root /shared/otter/references \
  --backend local
```

For BS-PDX or RNA-PDX, omit `--reference-primary` and provide both roles:

```bash
--reference-graft hg38@GRCh38.p14 \
--reference-host mm10@GRCm38.p6
```

There is no `--legacy`, `--species1`, or `--species2` authoring path.

The single-command equivalent is:

```bash
otter build \
  --project-root my_project \
  --fastq /data/fastq \
  --pdata /data/samples.csv \
  --mode RRBS \
  --reference-root /shared/otter/references \
  --reference-primary hg38@GRCh38.p14 \
  --backend local
```

`build` prints the absolute `run.yaml` path and stops before execution.

## 3. Plan, execute, and monitor

```bash
otter run \
  --config /absolute/project/runs/<run-id>/run.yaml \
  --executor craftmake \
  --phase step1 \
  --dry-run \
  --foreground

# Drop --dry-run and omit --foreground for a background task.
otter run \
  --config /absolute/project/runs/<run-id>/run.yaml \
  --executor craftmake \
  --phase step1

otter task list --all
otter task status <task-id>
otter task logs <task-id> --follow
otter task report <task-id>
otter task stop <task-id>
```

Do not override backend, phase resources, references, or input staging at run
time. Resolve a new snapshot when the contract changes.

## 4. Configure a SLURM site

```bash
otter site generate \
  --id production \
  --backend slurm \
  --partition cpu \
  --account genomics \
  --reference-root /shared/otter/references
otter site validate production

otter config resolve \
  --project my_project/project.yaml \
  --site production \
  --backend slurm
```

`site generate` may discover machine facts, but it does not invent cluster
policy. Partition and account/QOS come from the administrator.

## 5. Migrate an existing legacy project

Legacy support is read/validate/migrate only:

```bash
otter config validate \
  --config old_project/userspace/demo/config/otter.yaml \
  --schema legacy

otter init migrated
otter config migrate \
  --input old_project/userspace/demo/config/otter.yaml \
  --output migrated/project.yaml \
  --reference-root /shared/otter/references \
  --reference-primary hg19@GRCh37.p13-gencode-v19

otter config resolve \
  --project migrated/project.yaml \
  --reference-root /shared/otter/references \
  --backend local \
  --executor snakemake
```

Never run a raw `config/otter.yaml`. Both executors consume only an immutable
`otter.run/v1` snapshot.

## 6. Reference and artifact operations

```bash
# Publish a reference release.
otter reference build --id hg38 --release GRCh38.p14 \
  --organism "Homo sapiens" --assembly GRCh38 \
  --fasta genome.fa.gz --gtf genes.gtf.gz

# Promotion is preview-only without --confirm.
otter reference promote --help

# Verify/compare run publications.
otter artifact verify runs/<run-id>/run.yaml
otter artifact compare runs/<run-a>/run.yaml runs/<run-b>/run.yaml
```

Use `otter acquisition publish`, `otter benchmark collect`, and
`otter assets stamp|verify|print` only for their named evidence/asset
contracts; inspect `--help` before scripting them.

## 7. Failure routing

| Symptom | Inspect first | Corrective action |
| --- | --- | --- |
| Unknown `--legacy`/species/genome flag | `otter create --help` | Use reference roles; migrate old projects. |
| `run snapshot drift detected` | project/reference/input digest in `run.yaml` | Restore pinned intent or resolve a new run. |
| Immutable override rejected | changed run-time flag | Edit project/site intent and resolve again. |
| `tool_invocation` | task `stderr.log` and enva discovery | Verify `HOME`, `ENVA_RATTLER_ROOT_PREFIX`, and tool availability in `otter-core`. |
| SLURM submission failure | `craftmake doctor --backend slurm`, controller log, `squeue`/`sacct` | Fix site policy or resolve a new resource envelope. |
| Missing/invalid artifact | publication manifest and checksum verifier | Fix the producer; do not mark the run complete manually. |
| Legacy config passed to run | boundary error | Validate, migrate, resolve, then execute the snapshot. |

