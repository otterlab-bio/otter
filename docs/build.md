# Build guide

This guide builds the root CLI and its independent component repositories. It does not claim that compiling Craftmake completes the Otter migration.

## Prerequisites

- Go 1.24 or newer.
- Rust toolchain compatible with each locked submodule dependency set.
- Git 2 or newer.
- HDF5 development libraries for `methx`.
- Recommended environments: `go-env` for Go and `rust_build` for Rust.

## Build the root CLI

```bash
conda activate go-env
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o otter .
go test -count=1 ./...
go vet ./...
./otter --help
```

The output filename is `otter`, but source compatibility symbols may still contain historical `xdxtools` names. Verify the command help and version before packaging.

## Build the installer

The release installer is a standalone Go module in `installer/` that uses only the
standard library, so it builds as a fully static binary:

```bash
conda activate go-env
(cd installer && CGO_ENABLED=0 go build -trimpath -o otter-install .)
file installer/otter-install   # expect "statically linked"
```

The installer downloads pre-built static binaries from GitHub Releases, creates
conda environments, verifies pinned tool versions, and can run the Craftmake
ReferenceBuild workflow. It is published to
[`otterlab-bio/otter`](https://github.com/otterlab-bio/otter).

## Build the execution and environment layers

```bash
conda activate go-env
(cd craftmake && CGO_ENABLED=0 go build -trimpath -o ../target/bin/craftmake .)

conda activate rust_build
(cd enva && cargo build --locked --release)
```

Craftmake and Snakemake remain dual-track. A successful Craftmake build proves the execution layer compiles; it does not prove that every root workflow is dispatched through Craftmake.

## Build operators

| Component | Language | Output | Special dependency |
| --- | --- | --- | --- |
| `fastqcx` | Rust | `fastqcx` | FASTQ fixtures and compatibility checks |
| `xenofilx` | Go | `xenofilx` | BAM/reference fixtures |
| `pairbam` | Go | `pairbam` | BAM fixtures |
| `seq2mat` | Go | `seq2mat` | HTSeq mapping data |
| `matsrun` | Go | `matsrun` | rMATS runtime |
| `qctb` | Rust | `qctb` | report fixtures |
| `methx` | Rust | `methx` | HDF5 |
| `bamdriver` | Go | package/library | BAM fixtures |

Use the [submodule build guide](submodules-build-guide.md) and each component README for the authoritative entrypoint. Submodule layouts and release commands are independently versioned.

## Environment assets

Release verification must distinguish:

```text
otter-core
otter-snakemake
otter-extra
```

`otter-snakemake` is the compatibility runtime during migration. Search generated packages and release manifests for historical environment prefixes before publication.

## Validation matrix

Root checks:

```bash
go test -count=1 ./...
go vet ./...
./otter --help
```

Component checks should use each submodule's own CI contract. In general:

- Go repositories: `go test ./...`, `go vet ./...`, and supported static/race checks.
- Rust repositories: `cargo fmt --check`, `cargo check --locked`, `cargo clippy --locked --all-targets -- -D warnings`, and `cargo test --locked`.
- Workflow integration: the approved Snakemake compatibility and Craftmake contract/canary suites.

## Release wording

Until the bounded migration evidence and release checklist are satisfied, release notes must say `dual-track` or `migration in progress`. Do not announce universal Snakemake replacement based on a successful build or local contract test.

[Back to the documentation hub](README.md)
