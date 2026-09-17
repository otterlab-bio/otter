# Otter documentation hub

This directory holds the current user-facing documentation: the task-oriented manual, a complete set
of worked examples, and the JSON Schemas. Every command and path here is checked against the owning
repository's source before it is published.

<p align="center">
  <img src="../figs/otter-run-lifecycle.svg" width="100%" alt="Otter run lifecycle from project intent and inputs through immutable resolution, execution, verification, and publication">
</p>

## Start here

| Need | Read |
| --- | --- |
| Install a release or source checkout | [Installation](manual/01-installation.md) |
| Run a first workflow | [Quick start](manual/03-quickstart.md) |
| Prepare FASTQ inputs and sample metadata | [Data preparation](manual/02-data-preparation.md) |
| Choose a workflow scenario | [Analysis modes](manual/04-analysis-modes.md) |
| Resolve canonical files into an immutable run | [Advanced usage](manual/05-advanced-usage.md) |
| Migrate a legacy project to canonical v1 | [Reference migration](manual/08-reference-migration.md) |
| Look up a bundled operator or CLI | [Subtools](manual/06-subtools.md) |
| Troubleshoot a failure | [FAQ](manual/07-faq.md) |
| Look up the complete `otter` command tree | [CLI reference](manual/09-cli-reference.md) |

## The manual

The manual is the task-oriented sequence. Read it in order for a first run, or jump to a chapter:

1. [Installation](manual/01-installation.md) — release and source install, runtime environments, verification.
2. [Data preparation](manual/02-data-preparation.md) — FASTQ pairing rules, pdata format, sample naming.
3. [Quick start](manual/03-quickstart.md) — `init → create → config resolve → run` for both tracks, plus `otter build`.
4. [Analysis modes](manual/04-analysis-modes.md) — RRBS, WGBS, RNA-seq, BS-PDX, and RNA-PDX.
5. [Advanced usage](manual/05-advanced-usage.md) — site profiles, backends, run overrides, task control.
6. [Subtools](manual/06-subtools.md) — the bundled operators and when each one runs.
7. [FAQ](manual/07-faq.md) — common failures and what they mean.
8. [Reference migration](manual/08-reference-migration.md) — the legacy archive and the registry contract.
9. [CLI reference](manual/09-cli-reference.md) — project, site, reference, artifact, acquisition, and benchmark commands.

## Reference material in this directory

- [Worked examples](examples/) — a complete canonical project, reference release, lock file, run snapshot, and samples manifest.
- [Schemas](schema/) — the JSON Schemas for project configuration, run snapshots, references, locks, and artifact manifests.

## Interactive walkthroughs

- [OTTER on Google Colab](../notebooks/otter-colab-walkthrough.ipynb) — installs the published release with the real `otter-install`, writes a simulated reference registry, authors every scenario from the repository's own fixtures, and resolves every phase of each run with `--dry-run`. Offline after the install, and it executes no tool.
- [OTTER on Google Colab (简体中文)](../notebooks/otter-colab-walkthrough-zh.ipynb) — the same walkthrough with Chinese prose. The execution logic is identical; only the explanatory text differs in language.

  Open either directly in Colab:

  ```text
  https://colab.research.google.com/github/otterlab-bio/otter/blob/main/notebooks/otter-colab-walkthrough.ipynb
  https://colab.research.google.com/github/otterlab-bio/otter/blob/main/notebooks/otter-colab-walkthrough-zh.ipynb
  ```

  Both explain why the reference genome is simulated rather than downloaded (a real release is tens of gigabytes against roughly 100 GB of ephemeral Colab disk) and how to fetch a real one on your own hardware.

## Contracts outside this directory

These are authoritative for their own subject and are not duplicated here:

- [Offline e2e rehearsal](../scripts/e2e/otter_e2e.sh) — the authoring, build, site-profile, pairing, and migration contracts, asserted against the real binaries without network access.
- [Otter repository skill](../skills/otter/SKILL.md) — repository boundaries, toolchain matrix, validation strategy, and safe operating rules.
- [Craftmake workflow skill](../skills/craftmake/SKILL.md) — controller usage, YAML workflow authoring, and compatibility boundaries.
- [Workflow catalog](../craftmake/workflows/) — the Craftmake workflow families and their phases.

## Documentation rules

- Current product docs use `otter`, `craftmake`, `enva`, and the current operator names.
- External standards and scientific names such as FASTQ, FastQC, MultiQC, Bismark, Methrix, HTSeq, rMATS, BAM, and HDF5 are not renamed.
- Historical source symbols and old asset names such as `xdxtools`, `fastqc-rs`, `xenofilter-go`, `Paireads`, `htseq2matrix-go`, `gomats`, `methrix-cli`, and `bamdriver-go` are named only where compatibility requires them.
- Deferred work stays explicitly deferred and is never promoted to a release claim without a new evidence boundary.
- Every command and output path in a tutorial must be checked against the owning repository's current source before it is published.

## Documentation maintenance checklist

When changing a user-facing CLI or output contract:

1. Update the owning submodule README and the root component reference if the integration boundary changes.
2. Update the relevant manual chapter and link it from this hub.
3. Update `skills/otter/` when the build matrix, repository map, or safe operating rules change.
4. Run the README image and link audit, plus a repository-wide link and terminology check.
5. Keep release limitations visible near the first-use path.
