# Otter–Craftmake execution contract

This document defines the machine boundary between Otter and Craftmake. It describes the current canonical interface and keeps the legacy Snakemake path explicit.

## Responsibilities

```text
otter project/config
        ↓
immutable run.yaml
        ↓
executor router
   ↙             ↘
Craftmake      Snakemake compatibility
        ↓
Local or SLURM backend
```

- Otter owns project/configuration resolution, task records, process lifecycle, and top-level user commands.
- Craftmake owns workflow compilation, scheduling, task attempts, cache decisions, backend submission, SQLite state, controller JSONL, and reports.
- Snakemake is an explicit compatibility executor. It is not a failure fallback.

## Canonical Craftmake input

Craftmake reads one resolved snapshot:

```bash
craftmake validate \
  --config runs/<run-id>/run.yaml \
  --phase step1 \
  --catalog workflows/ \
  --format json

craftmake plan \
  --config runs/<run-id>/run.yaml \
  --phase step1 \
  --catalog workflows/
```

`run.yaml` is the source for scenario, workflow identity, backend, resources, references, inputs, and run-local paths. Craftmake does not read `project.yaml`, `samples.tsv`, a reference registry, or legacy configuration to fill in missing values.

## Execute, inspect, and recover

```bash
craftmake run \
  --config runs/<run-id>/run.yaml \
  --phase step1 \
  --backend local \
  --max-parallel 4 \
  --max-cores 16 \
  --max-memory 64G \
  --gate

craftmake status \
  --state workflow/.craftmake/state.sqlite \
  --run <run-id>--step1

craftmake logs \
  --state workflow/.craftmake/state.sqlite \
  --run <run-id>--step1

craftmake report \
  --state workflow/.craftmake/state.sqlite \
  --run <run-id>--step1 \
  --output reports

craftmake resume \
  --state workflow/.craftmake/state.sqlite \
  --run <run-id>--step1
```

Use `craftmake doctor --backend local` or `craftmake doctor --backend slurm` before execution. A Slurm run may remain briefly in `COMPLETING`; inspect both Craftmake state and `sacct`.

### Run identity is phase-scoped, and `--gate` is what makes it so

Craftmake keys exactly one row per run identity in `state.sqlite`
(`runs.run_id` is the primary key; `phase` is a column, not part of the key).
Without `--gate` Craftmake records the plain `<run-id>`, which means:

- a second phase of the same snapshot collides on the primary key, and
- `craftmake resume --run <run-id>--<phase>` finds no rows.

With `--gate` Craftmake records `<run-id>--<phase>` and enforces the immutable
backend, which is the identity Otter correlates task records against. Otter
therefore always passes `--gate` on `craftmake run`. `craftmake plan` writes no
run row, so a dry run correctly claims no identity. A repeated execution of an
already-recorded phase fails on the primary key; recover with `--resume` or by
resolving a new run.

### Step environments are resolved by `enva` at execution time

A workflow step declares an environment name (`environment: otter-core`), and
Craftmake resolves it through `enva` unless `CRAFTMAKE_ENV_PREFIX` names an
absolute prefix. `enva` discovers named environments from root prefixes derived
from `ENVA_RATTLER_ROOT_PREFIX`, `RATTLER_ROOT_PREFIX`, `MAMBA_ROOT_PREFIX`,
`CONDA_PREFIX`, and the current user's `$HOME` (`~/.local/share/rattler`,
`~/.local/share/mamba`, `~/.conda`).

Two consequences follow, and both are easy to misdiagnose:

- Discovery is `$HOME`-relative. A runner that overrides or sanitizes `HOME`
  (a per-job scratch home, `sudo`, cron, a service unit) can lose the ability to
  find an environment that plainly exists. Pass the real `HOME`, or set
  `ENVA_RATTLER_ROOT_PREFIX` to the root that holds the environment.
- A missing environment surfaces deep in Craftmake as a `tool_invocation`
  failure with exit code 1; the identifying message is in the step's
  `stderr.log` under the task runtime directory, not in Otter's output. Otter
  prints that directory and the controller log path when a run fails.

Select site profiles with `OTTER_SITE_PROFILE` or the default search path rather
than by overriding `HOME`, precisely because `HOME` also steers environment
discovery.

## Output formats

Commands support human-readable text and versioned JSON/JSONL envelopes through `--format`. The envelope identifies the command, success state, run ID, state database, controller log, and command payload. Human diagnostics remain on stderr and are not a durable state protocol.

The controller log records timestamps, run/task/submission IDs, backend job IDs, status, reasons, and metrics references. Reports export task timing, allocation information, cache decisions, and an evidence bundle.

## Identity and correlation

```text
Otter task ID
  → executor run ID
    → submission ID
      → task attempt ID
        → SLURM job/step ID
          → result + controller log + manifest
```

Changing samples, references, workflow assets, executor, toolchain, or immutable resources requires a new run. Resume is for recovering the existing contract, not for mutating it.

## Cancellation

```bash
craftmake cancel \
  --state workflow/.craftmake/state.sqlite \
  --run <run-id>--step1
```

Craftmake cancels active backend submissions, records the request and resulting status, and preserves controller evidence. Otter task cancellation also terminates the associated local process group when appropriate.

Every addressing command (`status`, `logs`, `report`, `resume`, `cancel`) takes
`--state <state.sqlite>` and `--run <run-id>--<phase>`. `run`, `plan`, and
`validate` take `--state-dir` instead.

## Artifact boundary

A successful workflow publishes declarations and then a create-only `results/artifacts.json` manifest. The manifest binds:

- immutable run ID;
- run snapshot digest;
- scenario, toolchain, executor, and backend;
- artifact paths relative to `results/`;
- checksums and declared comparison tiers.

Manifest verification confirms identity and file integrity. It is necessary for publication but is not, by itself, scientific parity.

## Failure classification

Failed or cancelled attempts may produce `otter.runtime-incident/v1` evidence with a stable category, retry policy, owner, backend identifiers, exit information, and bounded diagnostic paths. Human-readable stderr is retained as diagnosis, not used as the classification key.

Automatic retry is limited to bounded transient conditions such as selected scheduler submission or acquisition failures. Unknown internal incidents must be classified before promotion.

## Current status

The protocol and canonical snapshot boundary are implemented and covered by local contract tests. Real Slurm and paired executor evidence is accepted only within the bounded Gate 6 scope. Production-scale qualification and complete Snakemake replacement remain separate decisions.

[Back to the documentation hub](README.md)
