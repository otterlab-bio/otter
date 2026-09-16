# Otter workflow catalog

The catalog is the contract for scenarios, phases, toolchains, and published artifacts. It is not a claim that every listed path has passed production-scale scientific qualification.

## Current evidence labels

- **Implemented** — source and local contract coverage exist.
- **Bounded evidence** — a defined canary, control, or comparison has been accepted.
- **Deferred** — intentionally not part of the current release boundary.
- **Compatibility** — retained for established Snakemake or external-format workflows.

## Scenario matrix

| Scenario | Craftmake assets | Snakemake assets | Current status |
| --- | --- | --- | --- |
| RRBS | `craftmake/workflows/BeaverBS/` | `inst/snakefiles/BeaverBS*.snakemake` | Implemented; bounded executor evidence; WGBS-specific qualification separate |
| WGBS | BeaverBS components with WGBS configuration | BeaverBS compatibility assets | Implemented as a contract scenario; scientific requalification deferred |
| RNA-seq | `craftmake/workflows/BeaverRNA/` | `inst/snakefiles/BeaverRNA*.snakemake` | Implemented; bounded executor/publication evidence |
| BS-PDX | `craftmake/workflows/BeaverPDX/` | `inst/snakefiles/BeaverPDX*.snakemake` | Implemented; bounded executor and classification evidence |
| RNA-PDX | `craftmake/workflows/BeaverRNASEQPDX/` | `inst/snakefiles/BeaverRNASEQPDX*.snakemake` | Implemented; bounded executor and classification evidence |

The formal Craftmake catalog is the current execution path. Snakemake assets remain available only through explicit compatibility routing and are retained under the root `inst/` compatibility/legacy asset directories. Historical executor and tool comparisons belong in `docs/archive/`, `docs/notes/`, and the Gate 6 evidence records; they are not additional jobs in the formal Craftmake workflows.

## Orthogonal dimensions

```text
scenario × executor × toolchain × backend
```

- `scenario`: `rrbs`, `wgbs`, `rnaseq`, `bs-pdx`, `rna-pdx`.
- `executor`: `craftmake` or explicit `snakemake` compatibility.
- `toolchain`: `modern` or `legacy-equivalent` where a comparison is meaningful.
- `backend`: `local` for contract tests; `slurm` for real workflow evidence.

Executor comparisons hold toolchain, input, reference, and parameters constant. Tool comparisons hold executor constant. Mixing both axes invalidates attribution.

## Phase contract

| Phase | Purpose | Typical outputs |
| --- | --- | --- |
| `ingest` | Input/sample/reference preflight | input manifest, preflight report |
| `qc_raw` | Raw FASTQ QC | per-sample metrics, summary |
| `prepare` | Adapter/trim preparation | cleaned FASTQ, metrics |
| `separate` | PDX species classification | graft/host/ambiguous counts, filtered inputs |
| `align` | Reference alignment | sorted/indexed BAM, alignment metrics |
| `quantify` | Methylation, expression, or splicing | coverage, matrices, event tables |
| `qc_final` | Aggregate scientific QC | workbook, HTML, JSON summary |
| `publish` | Integrity and manifest publication | `artifacts.json`, checksums |

A scenario may omit an inapplicable phase, but an executor must not rename a shared phase or silently change its artifact meaning.

## Scenario contracts

### RRBS and WGBS

```text
ingest → qc_raw → prepare → align → quantify → qc_final → publish
```

Both use the BeaverBS family. RRBS and WGBS remain separate scenarios because restriction-site semantics, adapter defaults, coverage expectations, and qualification evidence differ. `pairbam` and `bamdriver` apply to paired-BAM stages only.

### RNA-seq

```text
ingest → qc_raw → prepare → align → quantify → qc_final → publish
```

STAR and locked GTF/reference assets feed expression matrices and optional rMATS outputs. `fastqcx`, `seq2mat`, `matsrun`, and `qctb` are the modern operator path.

### BS-PDX and RNA-PDX

```text
ingest → qc_raw → prepare → separate → align → quantify → qc_final → publish
```

Both require explicit `graft` and `host` references. Xenofilx owns classification and filtered BAM/BAI publication. BS-PDX continues through Bismark/methylation; RNA-PDX continues through STAR/expression and optional splicing. The host-absent rule and XG-aware mixture evidence are documented in the Xenofilx benchmark records.

## Toolchain mapping

| Domain | Modern | Compatibility or legacy-equivalent | Comparison tier |
| --- | --- | --- | --- |
| FASTQ QC | `fastqcx` | FastQC-compatible output; MultiQC remains an external consumer | exact/structural |
| PDX separation | `xenofilx` | XenofilteR-compatible workflow | scientific |
| Paired BAM | `pairbam`/`bamdriver` | Paireads-compatible behavior | structural |
| Count matrix | `seq2mat` | HTSeq matrix path | scientific |
| Splicing | `matsrun` | rMATS orchestration | scientific |
| Methylation | `methx` | Methrix/R path | scientific |
| QC aggregation | `qctb` | legacy report path | informational/structural |
| CluBCpG, mHap, CCGG, insert length | no complete modern replacement | legacy-only extension | not parity-scoped |

Modern Methx HDF5 is a custom schema until exported with the documented R function; it must not be described as a native Methrix directory.

## Artifact contract

Publishers write `results/artifacts.json` using the schemas under [`schema/`](schema/). Each declaration binds the run ID, snapshot digest, scenario, executor, backend, relative artifact path, checksum, and comparison tier.

- `exact`: normalized output is identical.
- `structural`: format, fields, dimensions, order, and identity match.
- `scientific`: domain comparator and declared tolerance pass.
- `informational`: retained for review and does not block promotion.

A complete manifest proves publication integrity, not scientific parity. Missing comparators or unavailable files fail closed rather than being downgraded silently.

## Release boundary

Accepted Gate 6 evidence includes bounded Craftmake–Snakemake executor comparison, corrected Gate A–D classification evidence, and Methx/Methrix parity. The following remain deferred and are not current release claims:

- fresh seven-input legacy-equivalent matrix;
- `20 samples × 3 repeats` representative matrix;
- production-scale throughput and scheduler pressure;
- WGBS `SRR6373947` requalification;
- additional Snakemake interruption/recovery studies.

See the [benchmark plan](benchmark-plan.md) and [evidence register](gate6-closeout-evidence-register.json) for the accepted boundary.

[Back to the documentation hub](README.md)
