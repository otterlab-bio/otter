# Otter documentation hub

This directory is the source of truth for current user-facing contracts, operational guidance, and evidence boundaries. Documents are grouped by lifecycle rather than by the date they were written.

<p align="center">
  <img src="./otter-run-lifecycle.svg" width="100%" alt="Otter run lifecycle from project intent and inputs through immutable resolution, execution, verification, and publication">
</p>

## Start here

| Need | Read |
| --- | --- |
| Understand the product | `Project overview` |
| Install a release or source checkout | `Installation` |
| Run a first workflow | [User manual](manual/README.md) |
| Understand the execution stack | `Architecture` |
| Choose a workflow scenario | `Workflow catalog` |
| Resolve canonical files into an immutable run | `Configuration`, `execution contract` |
| Configure sites and references | `Site profiles`, `reference registry` |
| Build the root or submodules | `Build guide`, `submodule build guide` |
| Understand release readiness | `Release readiness` |
| Understand accepted Gate 6 evidence | `Gate 6 comparison`, `evidence register` |

## Current documentation layers

### Product and user guidance

- `Project overview` — product hierarchy, current/target runtime, scenarios, and repository layout.
- `Installation` — release/source install, environments, verification, and troubleshooting.
- [User manual](manual/README.md) — task-oriented tutorial sequence for new users.
- `Requirements` — current product requirements and boundaries.

### Contracts and reference

- `Architecture`
- `Configuration`
- `Execution contract`
- `Project layout`
- `Site profiles`
- `Reference registry`
- [Reference migration](manual/08-reference-migration.md)
- `Workflow catalog`
- [Schema directory](schema/)

### Engineering and operations

- `Build guide`
- `Submodule build guide`
- `Benchmark plan`
- `Paracloud operations`
- [Offline e2e rehearsal](../scripts/e2e/otter_e2e.sh) — the `init → create → config resolve → run` chain, the site-profile leg, and the executor pairing contract, run without network access.
- [Otter repository skill](../skills/otter/SKILL.md) — repository boundaries, toolchain matrix, validation strategy, and safe operating rules.
- [Craftmake workflow skill](../skills/craftmake/SKILL.md) — controller usage, YAML workflow authoring, and compatibility boundaries.

### Evidence and project decisions

- `Current context` — compact current-state handoff.
- `Gate 6 evidence register` — machine-readable acceptance boundary.
- `Gate 6 comparison report` — accepted parity and limitation summary.
- `Review index` — dated reviews and remediation evidence.
- `Archive` — historical implementation records retained as evidence.
- `Notes` — dated execution handoffs and working records.

## Documentation rules

- Current product docs use `otter`, `craftmake`, `enva`, and the current operator names.
- External standards and scientific names such as FASTQ, FastQC, MultiQC, Bismark, Methrix, HTSeq, rMATS, BAM, and HDF5 are not renamed.
- Historical review, archive, and dated note files preserve the terminology and claims that were true when the evidence was recorded.
- Any deferred Gate 6 work remains explicitly deferred and must not be promoted to a release claim without a new evidence boundary.
- Commands and output paths must be checked against the owning repository's current source before being copied into tutorials.

## Documentation maintenance checklist

When changing a user-facing CLI or output contract:

1. Update the owning submodule README and the root component reference if the integration boundary changes.
2. Update the relevant manual chapter and link it from this hub.
3. Update `skills/otter/` when the build matrix, repository map, or safe operating rules change.
4. Run the README image/link audit and a repository-wide link/terminology check.
5. Keep release limitations visible near the first-use path.
