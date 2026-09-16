# Note 2026-08-15: Gate 6 Step2 Execution and SLURM Controller Incident

> Follow-up to `docs/notes/2026-08-15-gate6-step2-submission.md`.

## What happened

- The six step2 controllers (`41458048`–`41458053`) began Bismark (RRBS) and STAR (RNA-seq/RNA-PDX) alignment. BAM outputs are being produced: RRBS projects show multiple Bismark BAMs under `work/bsmap/<species>/`; RNA projects show the STAR BAM; `human-rnaseq-SRR1039508` also produced its `expression/` matrices.
- **Incident (documented)**: `human-rnaseq-SRR018258` step2 (controller `41458051`) failed with a classified exit code 5. Its STAR `map_and_sort` task succeeded (job `41458056`), but the dependent `qualimap` (job `41458069`) and `count_expression` (job `41458068`) submissions hit a transient SLURM controller error:
  `srun: error: Unable to confirm allocation for job ...: Unexpected message received`.
  This is the same transient controller fault class previously recorded for SRA decode. The task attempt dirs contain only `srun-launch.err` and no worker result, confirming the allocation-confirmation failure rather than a workflow defect.
- **Recovery status**: the resume controller `41458109` completed successfully. It reused the cached STAR BAM and both `qualimap` and `count_expression` completed successfully for `SRR018258`.
- **Recovery complete**: the failed mm10 STAR outputs and temporary directories were first moved intact to `work/bsmap/mm10/failed-star-attempt-20260815T144515Z/`; no incident artifact was deleted. With partial outputs removed from the active directory, resume controller `41459914` finished with exit code 0. The fresh mm10 `map_and_sort` completed successfully (worker `41459917`, 9m49s), followed by successful mm10 `qualimap` (worker `41459966`, 3m39s); the resumed step2 run finished successfully.

## Running state

| Item | Status |
|---|---|
| all six step2 project runs | completed successfully after targeted resume recovery |
| rna-pdx mm10 step2 | completed successfully; failed STAR evidence retained |
| SRR018258 step2 resume | completed successfully |
| bs-pdx step1 | completed successfully (`controller-exit 0` at `2026-08-15T21:18:57Z`) |
