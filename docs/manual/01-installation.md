# 1. Installation

This chapter covers release installation, source builds, runtime environments, and a first verification for **OTTER** (**O**rchestrated **T**ranscriptomic, **T**umor-xenograft (PDX/CDX), and **E**pigenomic **R**eporting Workflow).

## Requirements

- Linux or macOS for source builds and local development; Linux with SLURM is
  the primary production target. The umbrella `otter-install` release asset
  shown below currently targets Linux; on macOS, use the source-build path
  unless a matching platform asset is present in the selected release.
- Git with recursive submodule support.
- Go 1.24+ for the root and Go submodules.
- Rust/Cargo for `enva`, `fastqcx`, `methx`, and `qctb`.
- `methx` release binaries include the HDF5 runtime through static linking; HDF5 environment variables are not required at runtime.
- Source builds of `methx` require the Rust toolchain and native build tools used by `hdf5-metno`.

## Release installation

The release installer is a statically compiled Go binary published to
[`otterlab-bio/otter`](https://github.com/otterlab-bio/otter):

The following command installs the Linux amd64 asset. For another Linux
architecture, choose the matching release asset rather than renaming this
binary.

```bash
curl -fsSL -o otter-install https://github.com/otterlab-bio/otter/releases/latest/download/otter-install-linux-amd64-static
chmod +x otter-install
./otter-install
```

The legacy shell installer remains available for compatibility:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/otterlab-bio/otter/main/scripts/install.sh)
```

After installation, verify the selected binary and asset set rather than assuming the latest pre-release is production-qualified:

```bash
otter --help
otter --version
command -v otter
```

## Source installation

```bash
git clone --recurse-submodules https://github.com/otterlab-bio/otter.git
cd otter
conda activate go-env
go build -o otter .
./otter --help
```

If the checkout was cloned without submodules:

```bash
git submodule sync --recursive
git submodule update --init --recursive
git submodule status --recursive
```

The independent component directories are `craftmake`, `enva`, `fastqcx`, `xenofilx`, `pairbam`, `seq2mat`, `matsrun`, `qctb`, `methx`, and `bamdriver`.

## Runtime environments

Use `enva` to create the required core environment:

```bash
enva create --core
```

Create compatibility layers only when needed:

```bash
enva create --core --snakemake
enva create --core --extra
enva list --detailed
enva validate --all
```

Rust Bismark 3.1.0 and Bowtie2 2.5.5 are provided together by the `otter-core` environment. The installer no longer installs Bismark or Bowtie2 separately; run related commands through `enva run otter-core -- ...`.

| Environment | Purpose |
| --- | --- |
| `otter-core` | Required workflow dependencies and operators. |
| `otter-snakemake` | Optional Snakemake compatibility runtime. |
| `otter-extra` | Optional analysis and visualization tools. |

The production runtime remains dual-track. Install `otter-snakemake` for existing workflows and install/build `craftmake` for the canonical migration path.

## Verify component binaries

```bash
enva run otter-core -- bismark --version
enva run otter-core -- bowtie2 --version
enva run otter-core -- bismark_genome_preparation --help
```

The core specification pins `bismark=3.1.0` and `bowtie2=2.5.5`; the installer verifies these versions after environment creation. Bioconda provides this Bismark build for Linux x86_64, Linux aarch64, and macOS arm64; macOS x86_64 is not supported by the recipe.

Not every operator exposes `--version`; use `--help` for those tools.

## HDF5 build environment

Release binaries include HDF5 statically and do not require HDF5 runtime variables. When compiling `methx` from source, install the native build tools and let `hdf5-metno` build its dependency:

```bash
sudo apt-get install -y build-essential cmake pkg-config
cargo build --release
```


## Compatibility note

Some source-level symbols, generated state paths, or older release assets may still use `xdxtools`. That is a migration-era compatibility name; do not infer from a renamed output file that every code and runtime path has been renamed.

[Back to the manual](README.md) · [Documentation hub](../README.md)
