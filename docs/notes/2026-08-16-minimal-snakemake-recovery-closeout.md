# Note 2026-08-16: Minimal Snakemake Recovery Closeout

## Scope

The seven-input production comparison uses Craftmake only. Snakemake remains an
explicit compatibility path, so this closeout is deliberately isolated from the
comparison cells.

## Planned real-workflow evidence

- Resolve a new, immutable `otter.run/v1` for the completed
  `mouse-rrbs-SRR10025242` project with `--executor snakemake`.
- Submit a full explicit Snakemake compatibility run through Otter on Slurm.
- Cancel the controller in a controlled manner while the real workflow is
  active, then run the same immutable snapshot with `--resume`.
- Require successful artifact publication and `otter artifact verify` after
  recovery. Existing Craftmake snapshots and published artifacts remain
  untouched.

## First execution result

- Controller `41460152` reached Otter's real Snakemake compatibility path but
  stopped before workflow submission with `snakemake not found in PATH`.
- No Snakemake child job, workflow output, snapshot mutation, or artifact
  publication was produced. The failure is therefore an environment-preflight
  blocker, not evidence of a workflow or recovery failure.
- The next attempt must provide a sealed, compute-visible Snakemake executable
  through the controller environment before the controlled interruption is
  attempted.

## Asset staging recovery

- The failed snapshot `run-20260816T022709Z-mzmkvr` is retained as the
  missing-asset diagnostic record and will not be reused.
- The isolated project now contains the repository's version-controlled
  `inst/snakefiles` (at project root) and `inst/rules` assets, staged read-only.
  These are the assets required by the explicit Snakemake compatibility engine.
- A new snapshot must be resolved after staging so the root `*.snakemake` files
  and `rules/` directory are covered by the immutable workflow-asset digest.

## Controlled interruption

- Asset-sealed snapshot `run-20260816T024341Z-cijlua` launched controller
  `41460256`; its real Snakemake step1 child `41460261` entered `RUNNING`.
- The controller was then deliberately cancelled. Slurm accounting records
  `41460256` as `CANCELLED`, while `41460261` remained running at the time of
  cancellation. This is the required real controller-interruption state.
- The next controller will run the same immutable snapshot with `--resume` to
  reconcile that retained worker state and complete publication.

## Resume lock repair

- The recovery attempt exposed a stale `.snakemake` lock after controller
  cancellation. Otter's explicit Snakemake resume path now calls
  `snakemake --unlock` from the project directory before it re-enters the
  workflow manager.
- The unlock is limited to actual resume runs; dry-runs and normal Snakemake
  executions retain their prior behavior.
- `go test ./cmd` passes for the repair before deploying the closeout binary.

## Step2 runtime blocker

- The repaired resume completed Snakemake step1 and the step2 Bismark rule,
  publishing `SRR10025242_mm10.bam` and its BAI in the new run's isolated
  `work/bsmap/` directory.
- The following legacy Picard `CollectGcBiasMetrics` rule failed before it could
  read the BAM because it resolved Java from `MyMiniconda`; that binary fails on
  the compute node with `undefined symbol: JLI_StringDup`.
- This is a Java runtime ABI conflict in the compatibility controller
  environment. It does not affect the completed Bismark output, immutable run,
  interruption accounting, or the implemented stale-lock recovery behavior.

## Java home isolation repair

- The explicit Snakemake path now removes an inherited `JAVA_HOME` while it
  submits and monitors compatibility workflow children, restoring the original
  value when the command returns.
- This lets Picard use the Java bundled with its active Enva environment instead
  of a potentially incompatible controller-level JVM.
- The behavior is covered by `TestClearSnakemakeInheritedJavaHomeRestoresOriginalValue`;
  `go test ./cmd` passes.

## Step3 checker parse blocker

- The main step3 workflow completed, but the `step3-check` checker failed while
  Snakemake parsed `rules/methrix_object.smk`.
- Its shell block contains unescaped Bash parameter-expansion braces, so
  Snakemake treats expressions such as `${ref}.ron` as template variables and
  raises a `NameError` before any checker rule can run.
- This is a deterministic compatibility Snakefile formatting defect. The failed
  snapshot remains immutable evidence; a new snapshot will be resolved after
  staging a read-only corrected `methrix_object.smk` asset.

## Corrected asset snapshot

- The corrected `methrix_object.smk` was staged read-only in the isolated
  project and a fresh immutable snapshot was resolved:
  `run-20260816T035120Z-kfkzhm`.
- The failed snapshot `run-20260816T024341Z-cijlua` remains untouched and is
  retained as diagnostic evidence; the corrected run will be a fresh full
  compatibility execution so its artifact manifest is tied to the repaired
  workflow-asset digest.

- Full corrected compatibility controller `41461228` was submitted against
  `run-20260816T035120Z-kfkzhm`; it uses the isolated static Otter binary and
  the sealed corrected workflow assets.

## Corrected compatibility completion

- Controller `41461228` completed successfully in 01:09:51. Its Bismark
  step2 worker `41461467`, methylation step3 worker `41461612`, and all-sample
  step3-check worker `41461633` each completed with exit code `0:0`.
- The corrected checker passed the formerly failing Snakemake template parse,
  extracted 21,867,837 mm10 CpG sites, and completed the real Methrix
  annotation workflow. It atomically published `methrix_data.h5`,
  `CpG_coverage.xlsx`, and `CpG_annotation_report.xlsx` before Otter collected
  the declared outputs.
- Otter published three immutable artifacts to
  `results/artifacts.json`: the Bismark summary HTML report, Methrix HDF5
  dataset, and QC summary workbook.
- The deployed static compatibility binary verified the exact immutable run
  with `otter artifact verify <run-yaml>` and returned `{"passed": true}`.
  This completes the isolated Snakemake compatibility/recovery validation.

## Reasoning

This supplies the required genuine Snakemake controller interruption/resume
and publication evidence without using Snakemake as a production comparison
executor or mutating any completed Craftmake run.
