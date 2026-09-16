# Otter project overview

<p align="center">
  <img src="../figs/logo.png" width="180" alt="OTTER Logo">
</p>

**OTTER** (**O**rchestrated **T**ranscriptomic, **T**umor-xenograft (PDX/CDX), and **E**pigenomic **R**eporting Workflow) is the user-facing workflow CLI for RRBS, WGBS, RNA-seq, BS-PDX, and RNA-PDX analysis. It coordinates input validation, configuration, workflow execution, task control, and artifact verification across a collection of focused repositories.

## Product stack

```text
otter → craftmake → enva → operators → bamdriver
```

- `otter` owns project initialization, legacy input/configuration, canonical project resolution, executor routing, task control, embedded workflow assets, and user documentation.
- `craftmake` owns native workflow compilation, Local/SLURM scheduling, SQLite state, caching, recovery, cancellation, and reports.
- `enva` owns rattler-first environment lifecycle and explicit conda/mamba/micromamba compatibility.
- Operators own domain work: `fastqcx`, `xenofilx`, `pairbam`, `seq2mat`, `matsrun`, `qctb`, and `methx`.
- `bamdriver` supplies shared BAM/BGZF primitives to BAM-consuming operators.

## Two supported configuration paths

### Legacy-compatible project path

`otter init` and `otter create` generate the existing project layout and `otter.yaml`. Run it through the explicit Snakemake compatibility executor:

```bash
otter init my_project
otter create --fastq ./fastq --mode RRBS --pdata ./samples.xlsx \
  --output my_project/userspace --jobid demo_rrbs
otter run \
  --config my_project/userspace/demo_rrbs/config/otter.yaml \
  --executor snakemake --engine local --foreground
```

This is the current compatibility path for established production workflow assets. It must not be described as a Craftmake run.

### Canonical v1 path

A canonical project uses `project.yaml`, a samples manifest, reference locks, and an immutable `runs/<run-id>/run.yaml` snapshot:

```bash
otter config validate --config project.yaml --schema v1
otter config resolve --project project.yaml --backend local
otter run --config runs/<run-id>/run.yaml \
  --executor craftmake --phase step1 --backend local --foreground
```

The resolver records executor, backend, site, references, inputs, resources, workflow assets, and digests. Craftmake consumes the snapshot and rejects runtime overrides that would change its immutable contract.

## Workflow scenarios

| Scenario | Domain | Main outputs |
| --- | --- | --- |
| `rrbs` | Restriction-aware bisulfite sequencing | BAM/BAI, Bismark coverage, methylation HDF5, QC |
| `wgbs` | Whole-genome bisulfite sequencing | BAM/BAI, coverage, methylation HDF5, QC |
| `rnaseq` | STAR/HTSeq-compatible RNA-seq | BAM/BAI, counts, matrices, optional splicing, QC |
| `bs-pdx` | Graft/host bisulfite separation | classification, graft BAM/BAI, methylation outputs, QC |
| `rna-pdx` | Graft/host RNA separation | classification, graft BAM/BAI, matrices, optional splicing, QC |

The legacy `create --mode` interface selects `RRBS`, `WGBS`, or `RNASEQ`. Supplying `--species2` enables the two-species PDX shape.

## Repository layout

```text
otter/
├── cmd/                 CLI commands and executor routing
├── internal/            config, input, execution, workflow, task, and reference logic
├── inst/                embedded Snakemake assets and runtime files
├── testdata/            fixtures and end-to-end placeholders
├── docs/                current contracts, tutorials, operations, and evidence indexes
├── skills/              agent guidance for repository-aware work
└── <submodules>/        independent component repositories
```

The parent repository records submodule commits as gitlinks. A submodule README or code change is not part of the parent commit until the independent submodule commit is published and the parent pointer is deliberately updated.

## Current evidence boundary

Gate 6 accepted bounded executor comparison, corrected classification controls, and Methx/Methrix parity. Deferred work includes the fresh seven-input legacy-equivalent matrix, representative matrix repeats, production-scale throughput qualification, WGBS requalification, and additional Snakemake recovery studies. These limitations are release-language constraints, not missing links in the user tutorial.

## Documentation map

- [Documentation hub](README.md)
- [User manual](manual/README.md)
- [Architecture](architecture.md)
- [Configuration](configuration.md)
- [Workflow catalog](workflow-catalog.md)
- [Installation](installation.md)
- [Build and submodules](build.md)
- [Craftmake adoption](migration/craftmake-adoption.md)
- [Gate 6 evidence](gate6-toolchain-comparison-report.md)
