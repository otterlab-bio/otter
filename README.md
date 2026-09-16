<p align="right">
  <strong>English</strong> · <a href="./README_zh.md">简体中文</a>
</p>

<p align="center">
  <img src="./figs/logo.png" width="220" alt="OTTER Logo">
</p>

<h1 align="center">OTTER</h1>

<p align="center">
  <strong>Orchestrated Transcriptomic, Tumor-xenograft (PDX/CDX), and Epigenomic Reporting Workflow</strong><br>
  A reproducible bioinformatics workflow CLI for RRBS, WGBS, RNA-seq, and PDX analysis.
</p>

<p align="center">
  <a href="#what-you-can-do">Features</a> ·
  <a href="#quick-start">Quick Start</a> ·
  <a href="#the-workflow-stack">Workflow Stack</a> ·
  <a href="#components">Components</a> ·
  <a href="#documentation">Documentation</a>
</p>

---

Otter turns FASTQ inputs and sample metadata into validated workflow projects, then runs them locally or on SLURM with task tracking, resumability, and auditable outputs.

## Proof

The repository ships an offline rehearsal that drives the real binaries through the whole
authoring and execution chain — `init`, `create`, site generation, run resolution, and a dry-run
plan — for four scenarios, plus the legacy migration path and the executor pairing boundary. It
never downloads a genome: the registry is written to the production layout and then verified by the
production verifier.

```text
$ bash scripts/e2e/otter_e2e.sh --otter ./otter --craftmake ./craftmake
  stages passed: 64
  stages failed: 0
```

A real authoring pass, against a release written by that same offline fixture strategy:

```text
$ otter init demo
Pinned inst/snakefiles -> workflows/ (23 files)

$ otter create --output demo --fastq ./fastq --pdata ./samples.csv --mode RRBS \
    --jobid demo --reference-root ./registry --reference-primary hg19@GRCh37.p13-gencode-v19
Identified 2 samples
Successfully paired 2/2 samples
Verifying declared references against the registry...
Reference primary verified: hg19@GRCh37.p13-gencode-v19 (Homo sapiens, manifest sha256:...)
Canonical project created successfully!
```

The rehearsal uses a simulated release, so the digest above is elided. In the deployed registry this
selection resolves to `manifest sha256:33dfd7d4ec0a90c6e11fdc45d02b2d4e9b6d82e4a607148d1ab467a0e555accc`,
recorded with its compute-node verification in the Gate 6 operations record, which is a development-era artifact and is not part of the published tree.

<p align="center">
  <img src="./docs/otter-workflow-stack.svg" width="100%" alt="Otter workflow stack from project control through Craftmake, Enva, domain operators, and Bamdriver">
</p>

## What you can do

- Start a workflow project with `init` and generate a validated analysis configuration with `create`.
- Run RRBS, WGBS, RNA-seq, BS-PDX, and RNA-PDX workflows.
- Use local execution or SLURM, including per-step CPU, memory, and partition controls on the current Snakemake path.
- Track background runs with `task list`, `task status`, `task logs`, `task stop`, and `task report`.
- Resolve canonical project files into immutable `otter.run/v1` snapshots with reference and resource identity.
- Validate published artifact manifests and checksums at the run boundary.

## The workflow stack

```text
otter → craftmake → enva → operators → bamdriver
  │         │         │         │          │
  │         │         │         │          └─ shared BAM/BGZF primitives
  │         │         │         └─ fastqcx · xenofilx · pairbam · seq2mat
  │         │         │            matsrun · qctb · methx
  │         │         └─ rattler-first runtime environments
  │         └─ native Local/SLURM executor and task state
  └─ project, configuration, workflow, and task control plane
```

The runtime is intentionally dual-track. Existing production workflows use the Snakemake compatibility path; `craftmake` is the native Go execution layer being integrated and validated. Otter does not claim that Snakemake has already been removed.

## Evidence and release boundary

The accepted Gate 6 scope covers bounded executor comparison, corrected read/BAM classification evidence, and Methx/Methrix scientific parity. It does **not** claim production-scale throughput, a fresh seven-input legacy-equivalent matrix, complete WGBS qualification, or universal Snakemake replacement. See the [workflow catalog](docs/workflow-catalog.md). The Gate 6 evidence register is a development-era artifact and is not part of the published tree.

## Quick start

### Install a release

The release installer is a statically compiled Go binary published alongside the umbrella's
own releases:

```bash
curl -fsSL -o otter-install https://github.com/otterlab-bio/otter/releases/latest/download/otter-install-linux-amd64-static
chmod +x otter-install
./otter-install
```

The legacy shell installer remains available for compatibility:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/otterlab-bio/otter/main/scripts/install.sh)
```

### Build from source

```bash
git clone --recurse-submodules https://github.com/otterlab-bio/otter.git
cd otter
conda activate go-env
go build -o otter .
```

### Create and run a project

`otter init` and `otter create` default to the canonical v1 track, which is what `otter run` consumes.

```bash
otter init my_project

otter create \
  --output my_project \
  --fastq /data/fastq \
  --mode RRBS \
  --pdata /data/samples.csv \
  --jobid demo_rrbs \
  --reference-root /shared/otter/references \
  --reference-primary hg19@GRCh37.p13-gencode-v19

otter config validate --config my_project/project.yaml --schema v1
otter config resolve --project my_project/project.yaml --backend local \
  --reference-root /shared/otter/references

otter run \
  --config my_project/runs/<run-id>/run.yaml \
  --executor craftmake \
  --phase step1 \
  --backend local \
  --foreground
```

A reference is always selected as `<id>@<release>`. `create` verifies that selection against the
registry and writes the resolved identity to `references.lock.yaml`; `config resolve` then writes
the immutable `otter.run/v1` snapshot that `craftmake` executes.

The registry root is a deployment input, not project state: it is not stored in the project, because
the project is portable and the root is not. Supply it to **both** commands with `--reference-root`,
or set `OTTER_REFERENCE_ROOT`, or use a site profile that already declares it. Resolving without one
of those fails, and the verified digest in `references.lock.yaml` is what proves the release is the
same one. The executor, backend, phase, references, and resource envelope are fixed in
that snapshot; runtime flags cannot silently override it.

For a cluster run, replace `--backend local` with `--backend slurm` and supply the partition and
resource settings for your site. The default background mode prints a task ID:

```bash
otter task list
otter task status <task-id>
otter task logs <task-id> --follow
```

### Create and run a legacy-compatible project (compatibility track)

The Snakemake compatibility track requires `--legacy` on **both** authoring commands:

```bash
otter init my_project --legacy

otter create \
  --legacy \
  --fastq /data/fastq \
  --mode RRBS \
  --pdata /data/samples.xlsx \
  --output my_project/userspace \
  --jobid demo_rrbs

otter run \
  --config my_project/userspace/demo_rrbs/config/otter.yaml \
  --executor snakemake \
  --engine local \
  --foreground
```

Legacy `otter.yaml` is an authoring format that neither executor accepts directly; migrate and
resolve it first (see [reference migration](docs/manual/08-reference-migration.md)).

## Components

| Component | Role | Repository |
| --- | --- | --- |
| `otter` | Workflow project and task control plane | [otterlab-bio/otter](https://github.com/otterlab-bio/otter) |
| `craftmake` | Native workflow compiler and Local/SLURM executor | [otterlab-bio/craftmake](https://github.com/otterlab-bio/craftmake) |
| `enva` | Rattler-first environment lifecycle manager | [otterlab-bio/enva](https://github.com/otterlab-bio/enva) |
| `fastqcx` | FASTQ QC with FastQC-compatible summary output | [otterlab-bio/fastqcx](https://github.com/otterlab-bio/fastqcx) |
| `xenofilx` | Graft/host read classification for PDX data | [otterlab-bio/xenofilx](https://github.com/otterlab-bio/xenofilx) |
| `pairbam` | Paired-end BAM filtering and ordering | [otterlab-bio/pairbam](https://github.com/otterlab-bio/pairbam) |
| `seq2mat` | HTSeq count-to-expression-matrix conversion | [otterlab-bio/seq2mat](https://github.com/otterlab-bio/seq2mat) |
| `matsrun` | rMATS pairwise splicing orchestration | [otterlab-bio/matsrun](https://github.com/otterlab-bio/matsrun) |
| `qctb` | Versioned QC summary reporting | [otterlab-bio/qctb](https://github.com/otterlab-bio/qctb) |
| `methx` | Bismark coverage and methylation HDF5 processing | [otterlab-bio/methx](https://github.com/otterlab-bio/methx) |
| `bamdriver` | Shared pure-Go BAM/BGZF library | [otterlab-bio/bamdriver](https://github.com/otterlab-bio/bamdriver) |

> The links above follow the public repository naming contract. Historical source symbols and old asset names may still contain `xdxtools`, `fastqc-rs`, `xenofilter-go`, `Paireads`, `htseq2matrix-go`, `gomats`, `methrix-cli`, or `bamdriver-go`; those names are retained only where compatibility or historical evidence requires them.

## Documentation

- [Documentation hub](docs/README.md) — current contracts, tutorials, operations, and historical evidence map.
- [User manual](docs/manual/README.md) — installation, data preparation, quick start, modes, advanced usage, components, and FAQ.
- [Architecture](docs/architecture.md) — system boundaries and migration model.
- [Workflow catalog](docs/workflow-catalog.md) — scenarios, phases, artifacts, and comparison ownership.
- [Configuration and run snapshots](docs/configuration.md) — legacy and canonical configuration models.
- [Installation](docs/installation.md) — release, source, environment, and troubleshooting details.
- [Build and submodules](docs/build.md) · [submodule build guide](docs/submodules-build-guide.md).
- [Release readiness](docs/release-readiness.md) — current release checklist and deferred evidence boundary.
- [Methx → native Methrix HDF5 exporter](methx/scripts/export_methrix_hdf5.R) — R-side interoperability path.

## Development

```bash
conda activate go-env
go test -v ./...
go vet ./...
```

Rust submodules use the `rust_build` environment. Each submodule is an independent repository; see its own README for focused build and test commands.

## Naming note

The repository and product name is `otter`. Some source-level command and state symbols are still compatibility-era `xdxtools` names. Documentation distinguishes the current product name from those implementation aliases instead of pretending the rename is complete.

## License

MIT
