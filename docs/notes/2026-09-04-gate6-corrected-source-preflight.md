# Gate 6 corrected-source preflight — 2026-09-04

## Current gate position

The corrected-source prerequisite is complete for the accepted Gate 6 scope. Local validation, corrected binary deployment, read-level audits, CT/GA controls, and full Gate D classification accounting are accepted. The planned fresh seven-input modern-versus-legacy scientific comparison was not run and is now a deferred, non-blocking limitation; WGBS `SRR6373947` and additional Snakemake recovery work are likewise deferred. See the [Gate 6 closeout evidence register](../gate6-closeout-evidence-register.json).

## Corrected source validation

- `bamdriver/pkg/bamnative` tests passed.
- `bamdriver/cmd/nmoracle` tests passed.
- `go vet ./pkg/bamnative ./cmd/nmoracle` passed.
- `xenofilx/internal/classifier` tests passed.
- `go vet ./internal/classifier` passed.
- Static binaries were built with `CGO_ENABLED=0`.

Deployed runtime binary digests:

```text
gate6-nmoracle:             5b69ef7e55fd258862d01eacf1b359322a79dcb63ac79b482f63877045a33630
gate6-xenofilx-scoreaudit:  2c797a4425f8837aba39e8c9b374fe4f5ee510e1131e73b5b49a1e891182b9dd
```

The previous runtime binaries were copied to `gate6-corrected-backup-20260904/` before replacement. The corrected binaries were first uploaded to an isolated staging directory and then installed into the authorized runtime paths.

## Corrected read-level audits

| Job | Dataset | Mode | Input | Status |
|---:|---|---|---|---|
| `41965556` | human RNA-seq `SRR1039508` | conventional | hg38 BAM | completed; oracle parity passed |
| `41965558` | BS-PDX `SRR36187610` host | bisulfite | mm10 BAM | completed; oracle and CT/GA controls passed |
| `41965559` | BS-PDX `SRR36187610` graft | bisulfite | hg38 BAM | completed; oracle and CT/GA controls passed |

Each job uses a 200,000-record prefix, the corrected `gate6-nmoracle`, corrected `gate6-xenofilx-scoreaudit`, Picard 3.4.0 under the dedicated Java 17 prefix, and the existing comparison scripts. The BS-PDX jobs also generate explicit `XG=CT` and `XG=GA` converted-reference controls.

## Deferred fresh seven-input corpus preflight

The following corpus was prepared for a planned fresh parity phase, but that phase is outside the current Gate 6 release criteria and was not run:

```text
human RRBS     SRR31480456
mouse RRBS     SRR10025242
human RNA-seq  SRR1039508
human RNA-seq  SRR018258
mouse RNA-seq  SRR037954
BS-PDX         SRR36187610
RNA-PDX        SRR30880970
```

`SRR23802966` remains retired incident evidence and is not part of the fresh corpus. `SRR6373947` remains deferred for separate WGBS requalification.

Remote acquisition manifests were prepared for the seven-input corpus and for the retired `SRR23802966` record. No fresh seven-input legacy-equivalent matrix is claimed here. The existing canaries and RNA mixture benchmark remain bounded evidence only; reopening the deferred matrix requires separate scope authorization.

## Gate D provenance result

The provenance-corrected comparison is complete as job `41964124`. It uses the logical frozen run path `run-20260823T034624Z-ndcwfa` and reports:

```text
total input fragments: 45,160,030
modern graft fragments: 45,120,923
legacy graft fragments: 44,005,685
membership disagreements: 1,115,240
modern-only graft: 1,115,239
legacy-only graft: 1
```

The provenance path defect and corrected-source scientific classification accounting are closed. The remaining bounded modern/legacy semantic difference is recorded as a documented limitation in the closeout register.

The corrected read-level graft audit `41965559` has now completed successfully with `200,000/200,000` Xenofilx-vs-oracle equality and `200,000/200,000` consistency in both CT and GA control arms. Corrected Gate D modern preparation job `41967064` completed successfully in `01:38:23`; it processed `90,320,060` graft records and produced `90,241,846` filtered records with BAM SHA-256 `3e2c4b4c17c002be5f2a55e421bfc6364a5942a9a6c0b9165dd5a19709b05790`. Corrected legacy preparation job `41967065` completed successfully with `88,011,370` filtered records. Provenance-preserving membership comparison `41967198` completed with `44,005,684` graft/graft agreements, `39,106` discarded/discarded agreements, and `1,115,240` disagreements (`1,115,239` modern-only graft and `1` legacy-only graft), exactly matching the provenance-corrected baseline. The replacement stratification job `41972977` completed successfully in `00:06:14` against a deterministic sample of `10,000` modern-only disagreements. All sampled fragments had a complete graft primary pair and no legacy filtered primary records; `9,192` (`91.92%`) had no host primary alignment, while `808` (`8.08%`) had a complete host primary pair. The first stratification attempt (`41967199`) failed before reading data because of an obsolete evidence-directory prefix; it produced no scientific result. The controller was corrected. Score-decision audit `41973407` then completed successfully with no missing score records and the following decision pairs:

```text
9,190  modern=graft_host_absent;legacy=discarded_host_absent_both_mates_at_or_above_threshold
808    modern=graft_better_total;legacy=discarded_threshold
2      modern=discarded_threshold;legacy=discarded_host_absent_both_mates_at_or_above_threshold
```

The `808` both-mapped fragments therefore show a score/threshold difference, not a missing-mapping explanation. A full mapping stratification over all `1,115,239` modern-only disagreements completed as job `41973923`. The dependent full score-decision audit `41973925` also completed with no missing score records. The full scan closes Gate D accounting without extrapolating from the earlier sample.
