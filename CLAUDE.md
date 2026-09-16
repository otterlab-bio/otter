# CLAUDE.md

This file guides work in the `otter` parent repository.

## Project overview

`otter` (**OTTER**: **O**rchestrated **T**ranscriptomic, **T**umor-xenograft (PDX/CDX), and **E**pigenomic **R**eporting Workflow) is a Go bioinformatics workflow CLI for RRBS, WGBS, RNA-seq, and PDX analysis. The canonical GitHub repository is `otterlab-bio/otter`.

The product hierarchy is:

```text
otter → craftmake → enva → operators → bamdriver
```

`craftmake` is the Go replacement execution layer for Snakemake, but integration into `otter` is still migrating. Keep documentation and implementation accurate about the dual-track state: existing production workflows use Snakemake while `craftmake` adoption is validated. Do not claim the replacement is complete.

## Naming contract

| Current product | Historical name |
|---|---|
| `otter` | `xdxtools` / `xdxtools-go` |
| `fastqcx` | `fastqc-rs` |
| `xenofilx` | `xenofilter-go` |
| `pairbam` | `Paireads` |
| `seq2mat` | `htseq2matrix-go` |
| `matsrun` | `gomats` |
| `methx` | `methrix-cli` |
| `bamdriver` | `bamdriver-go` |

`craftmake`, `enva`, and `qctb` keep their names. Preserve external standards and scientific terms such as FASTQ, FastQC, MultiQC, Methrix, Bismark, HTSeq, and rMATS.

Current submodule directories are `craftmake/`, `enva/`, `fastqcx/`, `xenofilx/`, `pairbam/`, `seq2mat/`, `matsrun/`, `qctb/`, `methx/`, and `bamdriver/`. Treat each as an independent repository.

## Parent repository scope

- Go CLI: `main.go`, `cmd/`, `internal/`, `pkg/`
- Embedded runtime assets: `inst/`
- Tests: `testdata/` and `*_test.go`
- Current documentation: `README.md`, `README_zh.md`, `CLAUDE.md`, `docs/`
- Historical evidence: `docs/archive/` and dated files under `docs/review/`

Do not rewrite archived or dated review evidence merely to apply current branding. Add mappings in current indexes instead.

## Build and validation

```bash
conda activate go-env
go build -o otter .
go test -v ./...
go vet ./...
```

The root command and type names in the current source snapshot may still be `xdxtools` and `XDXToolsConfig`. The documentation contract names the product `otter` and the global configuration `OtterConfig`; until code migration is authorized, record the old symbols as compatibility aliases rather than silently claiming code has already changed.

For Rust submodules use `conda activate rust_build`. Do not modify a submodule while handling a parent-documentation-only task.

## Runtime environments

- `otter-core`: core bioinformatics runtime and operators
- `otter-snakemake`: Snakemake compatibility runtime during dual-track migration
- `otter-extra`: additional analysis and visualization tools

## CLI flow

```text
otter init → otter create → otter run → otter task/status
```

The logical data flow is:

```text
FASTQ/pdata → OtterConfig → craftmake or Snakemake compatibility path → enva → operators → bamdriver
```

## Conventions

- Go 1.24+
- Standard Go formatting and focused packages
- Stable explicit CLI flags
- Table-driven tests for parser and validator logic
- Test local and SLURM paths when execution changes
- Conventional Commit style when a commit is explicitly requested
- Never commit or push without explicit authorization
