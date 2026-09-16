# Otter project layout

The canonical v1 project separates versioned project intent from each immutable execution. Legacy projects may still contain the established `userspace/<jobid>/config/otter.yaml` layout and continue through explicit Snakemake compatibility.

## Which command writes what

A canonical project is composed from three commands, and none of them writes the whole thing alone:

| Command | Responsibility |
| --- | --- |
| `otter init` | Creates the canonical skeleton and pins the packaged assets: `workflows/`, `rules/`, `environments/`, `schemas/`, `runs/`, plus `project.lock.yaml` recording each asset set's digest. This is the track marker. |
| `otter create` | Writes project **intent**: `project.yaml`, `samples.tsv`, `references.lock.yaml`, after verifying every declared reference against the registry. |
| `otter config resolve` | Reads the project and writes one immutable `runs/<run-id>/run.yaml`. |
| `otter config migrate` | Converts a legacy `otter.yaml` into project intent. It does **not** create the asset skeleton, so its output must be written into an `init`-created root. |

`otter create --legacy` and `otter init --legacy` write the compatibility layout instead, and `.otter/assets.manifest.json` is that track's marker. A directory carrying both markers is ambiguous and is refused.

## Canonical layout

```text
project/
├── project.yaml
├── samples.tsv
├── references.lock.yaml
├── project.lock.yaml
├── workflows/
├── rules/
├── environments/
├── schemas/
└── runs/
    └── <run-id>/
        ├── run.yaml
        ├── manifest.json
        ├── input/
        ├── work/
        ├── results/
        ├── logs/
        ├── state/
        └── metrics/
```

The exact run subdirectories depend on the selected executor and workflow. The stable contract is that `run.yaml` is the resolved input and that results, logs, state, and metrics remain associated with the same run identity.

## Project-level files

| Path | Responsibility | Mutated by a run? |
| --- | --- | --- |
| `project.yaml` | User intent and defaults | No |
| `samples.tsv` | Versioned samples and FASTQ paths | No |
| `references.lock.yaml` | Logical reference releases and manifest digests | No |
| `project.lock.yaml` | Workflow, schema, and environment identity | No |
| `workflows/` | Project-pinned workflow assets | No |
| `rules/` | Compatibility rules where used | No |
| `environments/` | Enva declarations or locks | No |
| `schemas/` | Project-adopted schemas | No |

The resolver reads these files and writes a run snapshot. It must not depend on an unrecorded installation directory asset.

## Run identity

Canonical IDs use UTC time plus a random suffix:

```text
run-YYYYMMDDTHHMMSSZ-abcdef
```

A run ID is created atomically. Resume recovers the existing contract; it does not change the reference, samples, executor, toolchain, or workflow in place. A contract change requires a new run and, where applicable, lineage to the previous run.

## Run directories

| Path | Content |
| --- | --- |
| `run.yaml` | Resolved immutable execution configuration |
| `manifest.json` | Run, asset, tool, environment, and provenance summary |
| `input/` | Input identity and metadata; FASTQ is not copied by default |
| `work/` | Executor working files and temporary outputs |
| `results/` | Validated scientific outputs and artifact manifest |
| `logs/` | Otter, executor, submission, and task logs |
| `state/` | Task and executor state, including SQLite where applicable |
| `metrics/` | Timing, accounting, retry, and benchmark evidence |

## Isolation and publication

- Each benchmark or executor cell uses a distinct run ID.
- Craftmake and Snakemake must not share mutable work, state, or performance-sensitive caches.
- Scientific outputs are staged, validated, and published without overwriting an existing immutable result.
- The project root may keep an index or read-only latest pointer, but the run ID remains authoritative.
- Deleting evidence requires an explicit operation and provenance record.

[Back to the documentation hub](README.md)
