# Gate 6 closeout plan — 2026-09-05

> **Scope decision:** The accepted Craftmake–Snakemake comparison scope and corrected Gate A–D evidence are complete. The planned seven-input fresh legacy-equivalent matrix was not run and is not represented as completed evidence; it is no longer a Gate 6 release criterion. WGBS requalification, representative repeats, production-scale pressure testing, and additional Snakemake interruption/recovery work are explicitly deferred and non-blocking.

## 1. Objective and status boundary

This plan records the final Gate 6 closeout without conflating three evidence classes:

1. **Completed baseline evidence** — work executed before the Bismark `XG`-aware correction;
2. **Completed post-fix Xenofilx evidence** — corrected read-level audits, mixture benchmarks, and Gate D accounting;
3. **Accepted Gate 6 evidence** — real-Slurm Craftmake–Snakemake executor comparison, Gate A/B preservation, Gate C NM/oracle controls, Gate D disagreement accounting, and Methx/Methrix parity.

The comparison decision is complete for the accepted scope. The fresh seven-input modern-versus-legacy-equivalent matrix described in the earlier plan was not executed; its omission is an explicit scope limitation, not a hidden acceptance result. The retired BS-PDX `SRR23802966` line remains historical incident evidence only, and `SRR36187610` remains the active BS-PDX evidence line.

## 2. Priority order

The remaining work is documentation and evidence closeout only:

```text
P0 freeze scope and provenance             [complete]
  -> P1 corrected source validation        [complete]
  -> P2 Gate A/B BAM and pair integrity   [complete]
  -> P3 Gate C NM and CT/GA controls      [complete]
  -> P4 Gate D disagreement accounting    [complete]
  -> P10 evidence register and final report
```

The former P5–P9 work items are explicitly deferred, non-blocking extensions. They must not be restarted without new authorization and must not be described as completed Gate 6 evidence.

## 3. P0 — Freeze scope, inputs, and evidence contracts

### Actions

- Confirm the active seven-input corpus and mark `SRR23802966` retired.
- Confirm all active acquisition manifests are immutable `otter.sra-acquisition/v1` records.
- Confirm fresh `otter.run/v1` snapshots for every modern/legacy comparison cell.
- Freeze the corrected source revisions:
  - `bamdriver` XG-aware NM correction;
  - Xenofilx classifier wrapper and score-audit changes;
  - relevant runtime scripts and independent oracle.
- Create a single evidence register containing run ID, input digest, reference digest, workflow digest, tool digest, Slurm job IDs, and artifact locations.

### Deliverables

- [`gate6-closeout-evidence-register.json`](../gate6-closeout-evidence-register.json)
- frozen corpus list;
- frozen comparison-cell matrix;
- explicit distinction between pre-fix baseline and post-fix evidence.

### Stop condition

Do not submit corrected-source audit jobs if any binary, reference, input, or runtime script digest is ambiguous.

## 4. P1 — Validate corrected source and build deployable binaries

### Actions

In `bamdriver`:

- run formatter checks;
- run `go test ./pkg/bamnative ./cmd/nmoracle -count=1`;
- run `go vet ./pkg/bamnative ./cmd/nmoracle`;
- run the full test suite if the command channel permits;
- verify the independent oracle does not call the production NM implementation.

In `xenofilx`:

- validate the local bamdriver dependency revision;
- run classifier and score-audit tests;
- run `go mod verify` and `go vet` for affected packages;
- build static `gate6-xenofilx-scoreaudit`.

Build and record:

- `gate6-nmoracle`;
- `gate6-xenofilx-scoreaudit`;
- corrected Xenofilx production binary if Gate D requires a separate build;
- local SHA-256, `--version`, build flags, source revisions, and dependency graph.

### Deployment rule

Only after local validation passes:

1. back up the existing remote runtime binaries;
2. upload corrected binaries to the authorized runtime paths;
3. set executable permissions;
4. verify local and remote SHA-256 equality;
5. never overwrite historical evidence directories.

### Acceptance

- targeted tests pass;
- `go vet` passes for affected packages;
- local and remote binary digests match;
- runtime manifest records the corrected bamdriver commit and Xenofilx source revision.

## 5. P2 — Gate A/B real BAM and pair integrity

These checks are prerequisites for interpreting any downstream scientific difference.

### Gate A: bamdriver round-trip

For the active BS-PDX `SRR36187610` hg38 and mm10 BAMs:

```text
original BAM -> bamdriver decode/encode no-op -> roundtrip BAM
```

Compare canonical records by stable alignment identity, including occurrence ordinal:

```text
QNAME FLAG RNAME POS MAPQ CIGAR RNEXT PNEXT TLEN SEQ QUAL auxiliary fields and types
```

Record explicitly allowed changes only:

- BGZF layout;
- BAI byte layout;
- declared `@PG` addition;
- explicitly declared test tags.

### Gate B: pairbam integrity

For retained records and fragments, verify:

- no unexpected fragment loss, split, or duplication;
- read1/read2 identities are unchanged;
- mate pointers and TLEN are unchanged unless declared;
- secondary/supplementary membership is preserved;
- every retained output fragment traces to the expected input records.

### Deliverables

- canonical-record diff report;
- pair-level preservation report;
- structural checks: `quickcheck`, header, count, index, flagstat, stats;
- explicit allowlist and zero unexpected mutations.

### Acceptance

```text
unexpected record mutation = 0
unexpected record loss/duplication = 0
unexpected mate mutation = 0
BAM/BAI structural errors = 0
```

## 6. P3 — Corrected Gate C read-level NM and CT/GA controls

### Read-level audit cells

Run the corrected binaries on the frozen 200,000 mapped-record prefixes for:

1. human RNA-seq `SRR1039508`, conventional non-bisulfite;
2. BS-PDX `SRR36187610` host mm10, bisulfite;
3. BS-PDX `SRR36187610` graft hg38, bisulfite.

For each record, retain:

- stable alignment identity;
- original `NM:i`;
- Picard conventional NM;
- Picard bisulfite NM where applicable;
- independent conventional NM;
- independent XG/strand-aware bisulfite NM;
- corrected Xenofilx in-memory NM;
- insertion, deletion, mismatch, soft-clip components;
- final Xenofilx classification score;
- XG value and FLAG orientation.

### CT/GA converted-reference controls

Run separate forward/CT and reverse/GA controls for both BS-PDX references. Compare within the correct arm and identity universe rather than combining incompatible strand semantics.

### Acceptance

- corrected Xenofilx NM equals the independent corrected oracle for every audited record;
- corrected Xenofilx classification score equals the oracle score for every audited record;
- CT/GA arm comparisons match within each arm;
- conventional-versus-bisulfite differences are labeled as conversion semantics, not unexplained errors;
- zero reference lookup failures and zero malformed-alignment failures.

The earlier successful read-level jobs remain pre-fix/current-contract baseline evidence. They must be retained but not relabeled as corrected-source acceptance.

## 7. P4 — Corrected Gate D fragment-level comparison

### Fixed inputs and paths

Use identical original BS-PDX `SRR36187610` graft/host BAMs, references, thresholds, and fragment universe for both paths:

```text
legacy: Picard NM patch -> XenofilteR
modern: corrected Xenofilx --recalculate-nm --bisulfite
```

### Required accounting

For every input fragment, produce one normalized row containing:

- fragment ID and mate occurrence;
- modern classification;
- legacy classification;
- graft/host score components;
- graft/host MAPQ and CIGAR;
- selected species;
- threshold and classification reason;
- mapping and ambiguity state.

Stratify every disagreement into explicit categories:

- corrected bisulfite conversion semantics;
- threshold boundary;
- score tie;
- missing mate or unmapped fragment;
- ambiguous primary alignment;
- secondary/supplementary handling;
- BAM/pair preservation issue;
- Picard/XenofilteR implementation difference;
- unexplained.

### Acceptance

- all input fragments accounted for;
- all modern filtered records trace to input graft records;
- all disagreements assigned to a reproducible category;
- unexplained disagreement count is zero, or a documented scientific waiver is approved;
- the pre-fix `41,892,990` disagreement count is reported only as historical baseline.

## 8. P5 — Fresh seven-input modern-versus-legacy scientific parity (deferred)

The planned seven-input fresh legacy-equivalent matrix was not executed. It is removed from the current Gate 6 release criteria and must not be described as completed evidence. The accepted comparison decision is based on the completed real-Slurm Craftmake–Snakemake executor evidence, corrected Gate A–D audits, and Methx/Methrix parity. Reopening this matrix requires a new scope authorization.

## 9. P6 — Representative `20 samples × 3 repeats` matrix (deferred)

The representative repeat matrix is a non-blocking future extension. It is not required for the current Gate 6 closeout and must not be used to imply a production-wide reproducibility claim.

## 10. P7 — Production-scale throughput and scheduler-pressure (deferred)

Production-scale and scheduler-pressure testing is deferred as a future performance qualification. The historical `SRR23802966` OOM remains incident evidence only, and the Methx annotation benchmark must not be generalized into a whole-toolchain throughput claim.

## 11. P8 — WGBS `SRR6373947` requalification (deferred)

WGBS `SRR6373947` requalification is outside the current Gate 6 scope. Its reference and acquisition provenance remain a future work item and do not block this closeout.

## 12. P9 — Real Snakemake PDX interruption/retry and scientific comparison (deferred)

Additional Snakemake interruption, retry, resume, and post-recovery scientific comparison work is outside the current Gate 6 scope. The completed Craftmake–Snakemake comparison evidence is retained; any new recovery study requires separate authorization and a new evidence boundary.

## 13. P10 — Publication and final Gate 6 closeout

### Actions

- publish immutable artifact manifests for all accepted cells;
- update the consolidated Gate 6 report with post-fix versus pre-fix labels;
- link every conclusion to a stable evidence directory and digest;
- record open limitations, waivers, and deferred items;
- update the canary matrix and active context only after evidence directories are complete;
- keep submodule commits and root gitlink changes separate and reviewable.

### Final closeout package

- corrected-source runtime manifest;
- Gate A/B canonical preservation reports;
- Gate C read-level NM and CT/GA reports;
- Gate D fragment disagreement table and reason summary;
- Methx/Methrix CpG universe and matrix parity evidence;
- deferred-work register covering the unrun seven-input fresh matrix, representative `20 x 3`, production-scale testing, WGBS requalification, and Snakemake recovery;
- final Gate 6 decision log.

## 14. Deferred-work and closeout rules

The following items are explicitly deferred and non-blocking for the current Gate 6 scope:

- fresh seven-input legacy-equivalent scientific matrix, which was not run;
- representative `20 samples × 3 repeats` matrix;
- production-scale and scheduler-pressure qualification;
- WGBS `SRR6373947` requalification;
- additional Snakemake PDX interruption, retry, resume, and recovery comparison.

Current closeout work is limited to reviewing the evidence register, recording the final decision log, and performing BS-PDX publication/artifact verification only if formal publication is required. Historical baseline, incident, and pre-fix evidence must remain labeled as such. No deferred item may be described as completed evidence, and no repository may be committed or pushed without explicit authorization.
