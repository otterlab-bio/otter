---
name: craftmake
description: Use when authoring, validating, planning, running, recovering, or documenting Craftmake YAML workflows in the Otter ecosystem.
---

# Craftmake workflow skill

Craftmake is Otter's native workflow compiler and controller. Use it for canonical immutable runs, not as an implicit fallback for established Snakemake projects.

## Execution boundary

```text
otter project.yaml + samples.tsv + references.lock.yaml
  → immutable run.yaml
  → Craftmake controller
  → YAML catalog and compiled DAG
  → enva/operator commands
  → state.sqlite, logs, metrics, artifact manifest
```

The controller is the only supported execution control plane. It validates the immutable snapshot, compiles the DAG, submits local or SLURM tasks, persists state, reconciles scheduler accounting, supports resume/cancel, and publishes validated artifacts. Do not replace it with hand-written `sbatch` commands.

## Standard command sequence

```bash
craftmake doctor --backend local
craftmake doctor --backend slurm
craftmake plan \
  --config runs/<run-id>/run.yaml \
  --phase step1 \
  --catalog workflows/
craftmake run \
  --config runs/<run-id>/run.yaml \
  --phase step1 \
  --backend local \
  --max-parallel 4 \
  --gate
craftmake status --state runs/<run-id>/state/state.sqlite --run <run-id>--step1 --verbose
craftmake logs   --state runs/<run-id>/state/state.sqlite --run <run-id>--step1
craftmake report --state runs/<run-id>/state/state.sqlite --run <run-id>--step1 --output reports
craftmake resume --state runs/<run-id>/state/state.sqlite --run <run-id>--step1 --gate
craftmake cancel --state runs/<run-id>/state/state.sqlite --run <run-id>--step1
```

`--workers` and `--max-parallel` both cap active submissions: `--workers` takes
precedence, and `--max-parallel` is the fallback default when `--workers` is
zero. The two flag families are not interchangeable: `run`, `plan`, and
`validate` take `--state-dir`, while the addressing commands (`status`, `logs`,
`report`, `resume`, `cancel`) take `--state <state.sqlite>` plus `--run`.

For artifact verification, use the Otter command against the resolved run:

```bash
otter artifact verify runs/<run-id>/run.yaml
otter artifact compare runs/<run-a>/run.yaml runs/<run-b>/run.yaml
```

Run `doctor` before a SLURM submission. The selected backend, site, resources, references, workflow assets, and digests are fixed in `run.yaml`.

### Run identity is phase-scoped, and `--gate` is what produces it

Craftmake keys exactly one row per run identity in `state.sqlite`: `runs.run_id`
is the primary key and `phase` is only a column. Passing `--gate` makes
Craftmake record the phase-scoped identity `<run-id>--<phase>`; omitting it
records the plain `<run-id>`, which causes two concrete failures:

- a second phase of the same snapshot collides on the primary key
  (`UNIQUE constraint failed: runs.run_id`), so no multi-phase run is possible;
- `resume --run <run-id>--<phase>` finds no rows (`sql: no rows in result set`),
  so a failed run cannot be recovered.

`--gate` additionally enforces the immutable backend, run identity, and SLURM
resources, so overrides that `run.yaml` already fixed are rejected. Otter always
passes `--gate` on `run` for exactly this reason; a hand-typed `craftmake run`
must pass it too. `plan` writes no run row, so a dry run correctly claims no
identity.

A repeated execution of an already-recorded phase fails on the primary key.
Recover with `--resume`, or resolve a fresh run — do not delete state rows.

The mutable overrides (`--backend`, `--run-id`, `--partition`, `--account`,
`--qos`, `--time`, `--scratch-root`) remain available only when `--gate` is
absent. Treat that as exploratory use, never for a run whose results you intend
to accept:

```bash
craftmake run \
  --config runs/<run-id>/run.yaml \
  --phase step1 \
  --backend local \
  --partition compute
```

### Step environments are resolved by enva at execution time

A step declaring `environment: otter-core` is resolved through `enva` unless
`CRAFTMAKE_ENV_PREFIX` names an absolute prefix. `enva` discovers named
environments from `ENVA_RATTLER_ROOT_PREFIX`, `RATTLER_ROOT_PREFIX`,
`MAMBA_ROOT_PREFIX`, `CONDA_PREFIX`, and the current user's `$HOME`
(`~/.local/share/rattler`, `~/.local/share/mamba`, `~/.conda`).

Two consequences matter when a task exits 1 and the only classification is
`tool_invocation`:

- Discovery is `$HOME`-relative, so a runner that overrides or sanitizes `HOME`
  can lose access to an environment that plainly exists. Pass the real `HOME`,
  or set `ENVA_RATTLER_ROOT_PREFIX`.
- The identifying message is in the failing step's `stderr.log` under the task
  runtime directory inside `--state-dir`, not in the controller summary. Read
  `<state>/runs/<run-id>/tasks/<id>/attempt-001/steps/*/stderr.log`.

## ReferenceBuild

Craftmake can download, build, and publish an immutable reference genome release through the `ReferenceBuild` workflow. It reads a `reference-build.yaml` configuration and runs the `acquire_sources → prepare_assets → publish_release` DAG, which calls `otter reference build` to publish the standard registry directory.

```bash
craftmake plan \
  --reference-build-config \
  --config reference-build.yaml \
  --workflow workflows/ReferenceBuild/build.yaml \
  --phase build \
  --catalog workflows/

craftmake run \
  --reference-build-config \
  --config reference-build.yaml \
  --workflow workflows/ReferenceBuild/build.yaml \
  --phase build \
  --catalog workflows/ \
  --gate
```

The reference-build backend and partition are configurable in `reference-build.yaml`:

```yaml
reference_build:
  backend: local        # or slurm; default slurm
  partition: ""        # empty uses the site profile / --partition / CRAFTMAKE_SLURM_PARTITION
  # ... fasta/gtf URLs, checksums, registry_root, tool binaries ...
```

A default `reference-build.yaml` template ships in the release archive under `share/craftmake/configs/`. Pass `--gate` here for the same reason as any other run: it fixes the phase-scoped run identity and rejects resource overrides the configuration already settled.

## YAML workflow structure

A workflow document normally contains:

```yaml
name: BeaverBS step1
version: 1
on:
  otter:
    workflow: BeaverBS
    phase: step1
    modes: [RRBS, WGBS]
defaults:
  shell: bash
  environment: otter-core
jobs:
  job_id:
    name: Human-readable task name
    scope: sample
    dimensions: [sample]
    needs: [other_job_id]
    inputs:
      input_name: "${{ config.path }}"
    outputs:
      output_name: "${{ config.output }}"
    resources:
      cores: 4
      memory: 16G
      time: 01:00:00
      partition: compute
    env:
      KEY: value
    steps:
      - name: Execute one command group
        environment: otter-core
        run: |
          command --input '${{ inputs.input_name }}' --output '${{ outputs.output_name }}'
```

### Supported field contract

- `name`, `version`: required workflow identity.
- `on.otter.workflow`, `on.otter.phase`, `on.otter.modes`: catalog routing metadata.
- `defaults`: inherited shell, environment, and observability settings.
- `jobs`: named DAG nodes. Job IDs must be stable because they appear in task IDs and evidence.
- `scope`: controls expansion, normally `global` or `sample`.
- `dimensions`: expansion dimensions such as `sample` and `species`.
- `needs`: explicit job dependencies; input references to another job also establish dependencies.
- `inputs`, `outputs`: typed path contracts for the compiled task.
- `resources`: CPU, memory, time, and partition envelope. Immutable runs reject ad hoc changes.
- `env`: environment variables for the task shell.
- `steps`: ordered command groups inside one task.
- `steps[].environment`: optional step-level runtime boundary; otherwise the job/default environment applies.
- `steps[].run`: Bash command text after expression expansion.

## Expression namespaces

Expressions are resolved by the adapter/compiler from the loaded project or fixture context:

- `${{ config.* }}` reads workflow configuration values.
- `${{ sample.* }}` reads the current sample expansion.
- `${{ species.* }}` reads the current species expansion.
- `${{ inputs.* }}` references inputs of the current job.
- `${{ outputs.* }}` references outputs of the current job.
- `${{ jobs.<job_id>.outputs.* }}` references a dependency job's declared output.
- `${{ paths.* }}` references immutable run paths where supported by the adapter.
- `${{ resources.* }}` references the resolved resource envelope.

Keep path construction in declared inputs and outputs. Do not reconstruct reference paths from user-home paths or temporary directories inside shell commands.

## Modern toolchain rules

Formal Craftmake workflows use `otter-core` and the current operators:

- Bismark: `bismark`, `bismark_genome_preparation`, `bismark_methylation_extractor`, and report tools from `bismark=3.1.0`.
- Bowtie2: `bowtie2` and `bowtie2-build` from `bowtie2=2.5.5`.
- FASTQ QC: `fastqcx`; legacy FastQC/SeqKit comparison jobs belong in archived compatibility material.
- PDX separation: `xenofilx` owns classification and filtered BAM/BAI publication.
- Methylation output: `methx`; its custom HDF5 schema is exported to Methrix using the documented R adapter when needed.

Snakemake assets are retained as explicit compatibility archives. A `legacy-equivalent` workflow is comparison evidence, not the formal Craftmake production path.

## Compatibility matrix

| Feature | Craftmake formal workflow | Snakemake compatibility | Archive only |
| --- | --- | --- | --- |
| `name`, `version`, `on.otter` | Supported | Not a Snakemake-native contract | No |
| `defaults`, `jobs`, `inputs`, `outputs`, `resources`, `steps` | Supported | Translated through the adapter where applicable | No |
| `${{ config.* }}`, `${{ inputs.* }}`, `${{ outputs.* }}` | Supported | Fixture/adapter compatibility | No |
| Immutable `otter.run/v1` snapshot | Required for canonical runs | Not required for legacy projects | No |
| Local backend | Supported | Supported through explicit executor selection | No |
| SLURM backend and controller reconciliation | Supported | Explicit compatibility path | No |
| `legacy-equivalent` tool comparison | Not part of formal execution | Historical comparison path | Yes |
| Snakemake `Snakefile`, rule syntax, `shell:` blocks | Not parsed as Craftmake YAML | Supported by Snakemake executor | Compatibility |
| Hand-written `sbatch` orchestration | Unsupported | Unsupported as a replacement for controller | Yes |

## Authoring checklist

1. Choose a stable catalog family and phase.
2. Declare every input and output.
3. Make dependencies explicit with `needs` or job output references.
4. Use `otter-core` unless a documented optional environment is required.
5. Use version-pinned Bismark and Bowtie2 through the managed environment.
6. Keep shell commands reproducible and free of machine-specific absolute paths.
7. Validate and plan before running.
8. Test controller logs, state, metrics, publication, and failure recovery.
9. Keep comparison and legacy material under archive paths, outside the formal workflow catalog.

## Repository references

- Root integration: `docs/workflow-catalog.md`, `docs/reference-registry.md`, and `docs/execution-contract.md`.
- Craftmake implementation: `craftmake/README.md`, `craftmake/workflows/`, and `craftmake/internal/`.
- Immutable snapshot schema: `docs/schema/otter-run-v1.schema.json`.
