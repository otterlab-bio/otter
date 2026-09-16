# Configuration contract

Otter has two configuration families. Use the legacy family for established project directories and the canonical v1 family for new reproducible runs.

## Configuration flow

```text
project.yaml + samples.tsv
+ references.lock.yaml
+ site/profile + explicit overrides
                ↓
        otter config resolve
                ↓
       immutable run.yaml          <- the single execution boundary
                ↓
   executor selected IN the snapshot
        ↙                    ↘
craftmake                snakemake
                           (compatibility)

legacy otter.yaml ── otter config migrate ──→ project.yaml + samples.tsv
                                                 + references.lock.yaml
                                                 (then resolve as above)
```

The snapshot records which executor it was resolved for, and `otter run
--executor` must agree with that selection. A legacy `otter.yaml` is an
authoring format only: both executors consume `run.yaml`, so a legacy project
must be migrated and resolved before it can run. `otter config migrate` maps the
legacy compatibility track onto the Snakemake executor.

## Canonical project v1

A minimal project declares intent, samples, references, execution defaults, and resource defaults:

```yaml
schema_version: otter.project/v1
project:
  id: cohort-a
workflow:
  scenario: rrbs
  toolchain: modern
execution:
  executor: craftmake
  backend: auto
  site: auto
samples:
  manifest: samples.tsv
references:
  primary: hg38@GRCh38.p14
resources:
  defaults:
    cores: 8
    memory: 32GiB
```

The complete examples are in [`docs/examples/`](examples/).

## Samples manifest

Canonical samples use TSV with explicit paths and stable sample IDs:

```tsv
sample_id	r1	r2	group
S01	data/S01_R1.fastq.gz	data/S01_R2.fastq.gz	case
```

Rules:

- `sample_id` is unique and cannot contain path separators.
- Relative paths resolve from the project file and become normalized absolute paths in `run.yaml`.
- Inputs are identified by metadata and digest before execution.
- Changing samples requires a new run; resume does not reinterpret the project.

The legacy `create` command accepts Excel/CSV pdata with `sampleid` and `sample_group` or `condition`. See the [data preparation guide](manual/02-data-preparation.md).

## Resolution

```bash
otter config validate --config project.yaml --schema v1
otter config resolve \
  --project project.yaml \
  --reference-root /shared/otter/references \
  --backend slurm
```

Resolution performs strict schema parsing, sample/reference checks, site/backend selection, path expansion, digest calculation, and immutable snapshot writing. `--backend auto` may require an explicit resolved backend when the current site cannot prove a complete SLURM environment.

## Override precedence

```text
CLI override > site profile or detection > project.yaml > workflow defaults
```

The selected value and its source are recorded in the snapshot. Runtime environment variables must not silently change an already-written snapshot.

## Immutable run snapshot

A `run.yaml` records at least:

- run ID and creation time;
- project, samples, references, workflow, and environment digests;
- scenario, toolchain, executor, backend, and site with sources;
- resolved absolute input/reference paths and checksums;
- phase resources and run-local paths;
- observability, benchmark, parity, and artifact policies.

A run snapshot must not be edited in place. Create a new run when changing inputs, references, workflow assets, executor, toolchain, or resources.

## Legacy migration

```bash
otter init my_project_v1
otter config migrate \
  --from legacy \
  --to v1 \
  --input old/otter.yaml \
  --output my_project_v1/project.yaml \
  --reference-primary hg19@GRCh37.p13-gencode-v19 \
  --reference-root /shared/otter/references
```

Migration writes a canonical project, samples manifest, and reference lock only when the legacy fields are unambiguous. Conflicts and fields that cannot be inferred are reported; migration never silently chooses between incompatible references or sample interpretations.

Two constraints follow from the resolver contract:

- **Every reference must be named as `id@release`.** A legacy configuration records filesystem paths, which do not identify an immutable registry release, so migration cannot invent one. `--reference-root` lets migration resolve and lock the declared references; without it the migrated project has no `references.lock.yaml` and cannot be resolved.
- **The output must live in a canonical project root.** Migration writes project intent, not the pinned asset skeleton, so run `otter init` first and migrate into that root.

The migrated snapshot selects the **Snakemake** executor, because the legacy adapter maps the compatibility track onto Snakemake.

## QCTB and shared snapshots

QCTB can consume the same immutable run snapshot:

```bash
qctb --config runs/<run-id>/run.yaml --output qc_summary.xlsx
qctb --config-dir runs/<run-id> --output qc_summary.tsv --format tsv
```

`--config-dir` resolves only `<run-directory>/run.yaml`. It is not a general multi-file configuration search path.

## Related documents

- [Architecture](architecture.md)
- [Execution contract](execution-contract.md)
- [Reference registry](reference-registry.md)
- [Project layout](project-layout.md)
- [Configuration examples](examples/)
