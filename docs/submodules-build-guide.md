# Submodule build guide

The root repository records ten independent component repositories as Git submodules. Each submodule has its own history, tests, release cadence, and working-tree boundary.

```text
otter → craftmake → enva → operators → bamdriver
```

## Initialize a checkout

```bash
git clone --recurse-submodules https://github.com/otterlab-bio/otter.git
cd otter
git submodule sync --recursive
git submodule update --init --recursive
git submodule status --recursive
```

Do not run `git submodule update --remote --merge` as a routine build step. It changes the parent repository's recorded gitlink.

## Component matrix

| Layer | Directory | Language | Build output or role |
| --- | --- | --- | --- |
| Execution | `craftmake/` | Go | `craftmake` |
| Environment | `enva/` | Rust | `enva` |
| FASTQ QC | `fastqcx/` | Rust | `fastqcx` |
| PDX separation | `xenofilx/` | Go | `xenofilx` |
| Paired BAM | `pairbam/` | Go | `pairbam` |
| Count matrix | `seq2mat/` | Go | `seq2mat` |
| Splicing | `matsrun/` | Go | `matsrun` |
| QC aggregation | `qctb/` | Rust | `qctb` |
| Methylation | `methx/` | Rust | `methx` |
| BAM foundation | `bamdriver/` | Go | shared packages |

## Go submodules

```bash
conda activate go-env
export CGO_ENABLED=0
go test ./...
go vet ./...
go build ./...
```

Run these commands inside `craftmake/`, `xenofilx/`, `pairbam/`, `seq2mat/`, `matsrun/`, or `bamdriver/`. Check the component README when the repository exposes a command under `cmd/<name>` rather than the module root.

## Rust submodules

```bash
conda activate rust_build
cargo fmt --check
cargo check --locked --all-targets
cargo clippy --locked --all-targets -- -D warnings
cargo test --locked
cargo build --locked --release
```

Run these commands inside `enva/`, `fastqcx/`, `qctb/`, or `methx/`. `methx` release binaries statically include HDF5; source builds need only the native build tools used by `hdf5-metno`:

```bash
sudo apt-get install -y build-essential cmake pkg-config
cargo build --locked --release
```

## Install and verify binaries

```bash
install -d "$HOME/.cargo/bin"
install -m 755 <built-binary> "$HOME/.cargo/bin/<current-name>"
command -v craftmake enva fastqcx xenofilx pairbam seq2mat matsrun qctb methx
```

`bamdriver` is primarily a shared Go package and may not provide a standalone executable in every checkout.

## Dual-track validation

- `otter-snakemake` remains the compatibility runtime for established production assets.
- `craftmake` build success does not prove root integration or scientific parity.
- Before changing the default route, compare task graphs, resources, failure propagation, recovery, and key scientific artifacts under the same immutable input/reference contract.
- Real workflow and benchmark evidence belongs on the approved SLURM path; Local tests are contract evidence.

## Parent gitlink delivery

A submodule change must be tested and committed in the submodule repository before the parent pointer is updated. This documentation refactor changes no submodule code, gitlink, script, or CI configuration.

[Back to the documentation hub](README.md)
