# Site and backend profiles

Site resolution turns portable project intent into an execution-ready run snapshot. The selected backend, site, resource envelope, and evidence supporting that choice must be recorded before execution.

## Default selection

```yaml
execution:
  executor: craftmake
  backend: auto
  site: auto
```

Use `--backend local` for development or contract tests. Use `--backend slurm` for real workflow execution when the site is configured and visible from compute nodes.

## Backend detection

```text
backend=auto
     ↓
check sbatch/squeue/sacct/scancel/srun
     ↓
none available ─────────→ local
all available ──────────→ validate cluster and paths
partial/inconsistent ───→ fail closed
```

A complete SLURM toolchain is not enough. Detection also validates the cluster, selected partition/account/QOS, resource limits, shared project/reference paths, scratch policy, and compute-node visibility.

Otter must not silently choose Local when SLURM is partially available or when a production reference path cannot be proven accessible.

## Inspect the current environment

```bash
otter site list
otter site validate
otter site validate production-cluster
```

The validation result reports backend, site ID, source, reason, cluster, and detected commands.

## Profiles can be generated

`otter site generate` probes the environment and writes a profile for you:

```bash
otter site generate --id dev-local --reference-root /shared/otter/references
otter site generate --id production --backend slurm \
  --partition cpu112c --account genomics \
  --reference-root /shared/otter/references --scratch-root /scratch/genomics
```

Discovery reads what the machine can actually answer: which SLURM tools are on
`PATH`, the cluster name from `scontrol`, the advertised partitions from `sinfo
-o %P`, and the reference/scratch root candidates from `$OTTER_REFERENCE_ROOT`
and `$OTTER_SCRATCH_ROOT`.

Discovery cannot decide site **policy**. An account, a QOS, and which partition
is approved for a project are operator decisions, so `--backend slurm` requires
`--partition` and `--account` explicitly rather than inventing them. With
`--backend auto`, the backend becomes `slurm` only when the toolchain is complete
*and* both were supplied; otherwise it writes a valid `local` profile.

The generated profile is validated with the same rules the loader applies, so
`otter site generate` can never write a file that `otter site validate` would
reject. It refuses to overwrite an existing profile unless `--force` is passed,
because that profile may already carry a partition/account validated against the
cluster.

Profiles are discovered from these locations, in order:

1. `$OTTER_SITE_PROFILE` (an explicit file path)
2. `~/.config/otter/sites/<site-id>.yaml` (where `generate` writes by default)
3. `/etc/otter/sites/<site-id>.yaml`

Loading is strict: unknown fields are rejected, `schema_version` must be
`otter.site/v1`, `site.backend` must be `local` or `slurm`, a `slurm` profile
requires a `slurm` block, and any declared path must be absolute. A `slurm`
profile additionally requires `paths.reference_root`.

A generated profile is real configuration, not a note: resolving with `--site
<id>` and no `--reference-root` takes the registry root from the profile.

## Site profile

A profile can provide stable site defaults without embedding credentials:

```yaml
schema_version: otter.site/v1
site:
  id: production-cluster
  backend: slurm
slurm:
  partition: cpu
  account: genomics
  qos: normal
  max_jobs: 100
  default_time: 24:00:00
paths:
  reference_root: /shared/otter/references
  scratch_root: /scratch/otter
```

Profiles may define defaults and constraints. Authentication, tokens, and private credentials belong to the site's standard environment, not to committed YAML.

## What detection actually runs

`backend: auto` validates a profile against the live cluster; `otter site generate` collects the observable subset of the same facts to seed a profile:

| Check | Mechanism |
| --- | --- |
| SLURM toolchain present | `sbatch`, `squeue`, `sacct`, `scancel` all on `PATH` |
| cluster name | `scontrol show config` (`ClusterName=`) |
| partition exists | `sinfo -h -p <partition>` |
| account exists | `sacctmgr -s -n -P show assoc where account=… format=Account` |
| account↔partition | `sacctmgr … format=Account,Partition` |
| QOS exists | `sacctmgr … show qos where name=…` |
| reference/scratch paths | `test -d` and `test -r` on the login node |

A partial toolchain (1–3 of the four commands) fails closed rather than falling
back to local. Compute-node visibility is a separate preflight, run as
`srun --ntasks=1 --partition=… --account=… bash -c 'test -d … && test -r …'`,
and only for resolved Slurm runs.

## Resource precedence

```text
explicit CLI value > site profile/detection > project value > workflow default
```

For canonical immutable runs, the effective values are written to `run.yaml`. Craftmake then rejects runtime overrides that would change the snapshot. For legacy Snakemake runs, the existing `otter run` resource flags remain available:

```bash
otter run \
  --config my_project/userspace/<jobid>/config/otter.yaml \
  --executor snakemake \
  --engine slurm \
  --slurm-partition cpu \
  --slurm-cores 16 \
  --slurm-memory 64G
```

## Fail-closed rules

Resolution must stop before `sbatch` when:

- required SLURM commands are missing or inconsistent;
- the requested partition/account/QOS is unavailable;
- CPU, memory, time, or submission limits cannot satisfy the run;
- shared project, reference, or scratch paths are not visible where required;
- a reference manifest or workflow asset digest differs;
- an automatic choice cannot be explained deterministically.

The diagnostic should identify the candidate, rejected constraint, expected value, actual value, and affected path or resource.

## Snapshot stability

Site state can change after a snapshot is written. That must not mutate the existing run. Re-resolve to create a new run when the selected site, backend, partition, resource envelope, or reference visibility changes.

[Back to the documentation hub](README.md)
