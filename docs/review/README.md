# Release Review Workflow

## Purpose

`docs/review/` stores release gates, remediation plans, and evidence. Dated reports are historical records and intentionally retain the repository and binary names that were true when the evidence was produced.

## Current component map

The current parent repository has 10 submodules:

```text
craftmake enva fastqcx xenofilx pairbam seq2mat matsrun qctb methx bamdriver
```

Historical report mapping:

| Current name | Historical report name |
|---|---|
| `fastqcx` | `fastqc-rs` |
| `xenofilx` | `xenofilter-go` |
| `pairbam` | `Paireads` |
| `seq2mat` | `htseq2matrix-go` |
| `matsrun` | `gomats` |
| `methx` | `methrix-cli` |
| `bamdriver` | `bamdriver-go` |

`craftmake`, `enva`, and `qctb` retain their names. Preserve FastQC, MultiQC, Methrix, Bismark, HTSeq and rMATS when they refer to external standards, tools or scientific contracts.

## Current release review status

The review directory is historical evidence and planning material. The current release decision is summarized in [release readiness](../release-readiness.md); accepted Gate 6 evidence is indexed in the [evidence register](../gate6-closeout-evidence-register.json).


- Review plan: [`submodule_review_plan_2026-07-21.md`](submodule_review_plan_2026-07-21.md)
- Wave 1 remediation: [`wave1_remediation_plan_2026-07-21.md`](wave1_remediation_plan_2026-07-21.md)
- Wave 2/3 remediation: [`wave2_wave3_remediation_plan_2026-07-22.md`](wave2_wave3_remediation_plan_2026-07-22.md)
- Dated submodule reports: [`submodules/`](submodules/)

Do not rename or rewrite those dated files in bulk. New reviews should use current component names and may link the historical report that established the baseline.

## Execution migration gate

`craftmake` is the Go replacement execution layer for Snakemake, but `otter` integration remains dual-track. A release may only claim complete replacement after RRBS, WGBS, RNA-seq and PDX pass task-graph, resource, recovery and key-output equivalence checks across local/SLURM paths.

## Release evidence

The existing evidence scripts and dated bundles may still use historical product names because scripts/CI are outside a documentation-only migration. Preserve those artifacts until their implementation migration is separately authorized.
