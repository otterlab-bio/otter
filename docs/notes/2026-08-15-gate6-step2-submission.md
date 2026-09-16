# Note 2026-08-15: Gate 6 Step2 Submission and Phase Envelope Fixes

> Follow-up to `docs/notes/2026-08-15-gate6-step1-provenance-and-snapshots.md`.

## What changed

- **Step2 controllers submitted** for the six projects whose step1 completed: controllers `41458048` (human-rrbs), `41458049` (mouse-rrbs), `41458050` (human-rnaseq SRR1039508), `41458051` (human-rnaseq SRR018258), `41458052` (mouse-rnaseq SRR037954), `41458053` (rna-pdx), all running `otter run --foreground --executor craftmake --phase step2`.
- **Phase resource envelope fixes** discovered through plan compilation against the current workflow catalog:
  - `rna-pdx` step2 raised to 80 cores / 160 GiB / 12 h because `BeaverRNASEQPDX/step2/map_and_sort` requests 80 cores.
  - `human-rnaseq-SRR1039508`, `human-rnaseq-SRR018258`, `mouse-rnaseq-SRR037954` step2-check raised to 24 cores / 64 GiB / 8 h because `BeaverRNA/step2-check/rnaseq_splicing` requests 20 cores (and `construct_expression_matrix` requests 5).
- **Re-resolves** produced new authoritative `otter.run/v1` snapshots for the affected projects; the completed step1 outputs were preserved by symlinking each new run's `work/{trim,QC,fastqc_raw,fastqc_clean}` to the original step1 run's directories.
- **Plan verification** passed for every phase of the six completed projects: RRBS step2/step2-check/step3, RNA-seq step2/step2-check, rna-pdx step2/step2-check.

## Authoritative run IDs after this round

| Project | Run ID |
|---|---|
| human-rrbs-SRR31480456 | `run-20260815T093050Z-ykunpt` |
| mouse-rrbs-SRR10025242 | `run-20260815T093102Z-liwxbq` |
| human-rnaseq-SRR1039508 | `run-20260815T134115Z-fvhpxa` |
| human-rnaseq-SRR018258 | `run-20260815T134119Z-vdwxod` |
| mouse-rnaseq-SRR037954 | `run-20260815T134123Z-iusjfz` |
| rna-pdx-SRR30880970 | `run-20260815T132155Z-vhesje` |

## In progress

- `bs-pdx-SRR23802966` step1 controller `41457463` still running (legacy FastQC on 57 GB raw input under node contention); the autonomous recovery watcher `runtime/recover-bs-pdx.sh` remains active. After bs-pdx step1 completes, its step2/step2-check/step3 envelopes must be validated (BeaverPDX) before step2 submission.
