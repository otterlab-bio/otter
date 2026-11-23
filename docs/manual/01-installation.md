# 1. Installation

This chapter covers release installation, source builds, runtime environments, and a first verification for **OTTER** (**O**rchestrated **T**ranscriptomic, **T**umor-xenograft (PDX/CDX), and **E**pigenomic **R**eporting Workflow).

## Requirements

- Linux or macOS on amd64 (x86-64) or arm64 (AArch64/Apple Silicon). Linux with
  SLURM remains the primary production target.
- For source builds: recursive Git submodules, Go 1.26.3 or newer for the full
  toolchain, and stable Rust/Cargo with native C/CMake tools for the Rust CLIs.
  The umbrella Go module alone declares Go 1.24.

## Release installation

The [OTTER releases](https://github.com/otterlab-bio/otter/releases) built by the
four-platform workflow use these names for the Go installer. The other tools
have the same platform suffixes.

| Host | Installer asset | Linkage |
| --- | --- | --- |
| Linux x86-64 | `otter-install-linux-amd64-static` | Static |
| Linux AArch64 | `otter-install-linux-arm64-static` | Static |
| macOS Intel | `otter-install-darwin-amd64` | macOS system libraries |
| macOS Apple Silicon | `otter-install-darwin-arm64` | macOS system libraries |

The shell installer detects your platform and selects matching binaries:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/otterlab-bio/otter/main/scripts/install.sh) --skip-envs
```

Alternatively, download the matching Go installer asset from a release, make it
executable, and run it. Both installers accept `--version TAG` to select one
release. Older releases may not have binaries for your platform. Without a tag,
the installers look for a published release with the matching tool assets,
including daily prereleases. Use `--skip-envs` to install binaries without
creating the optional environment setup. On Intel macOS, both installers skip
environment setup automatically: the pinned Bismark package has no `osx-64`
build. This does not prevent installing or running the native OTTER tools.

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

The independent component directories are `craftmake`, `enva`, `fastqcx`, `xenofilx`, `pairbam`, `seq2mat`, `matsrun`, `qctb`, `methx`, `bamdriver`, and the container repository `holt`.

## Container image

The [`holt`](../../holt) submodule builds the containerized workbench, and its
`holt-run` image supersedes the old Dockerfile that used to live in this
repository root. The image resolves the newest stable OTTER release at build
time, so it follows this repository instead of pinning a version:

```bash
docker run --rm -it -p 2222:2222 ghcr.io/otterlab-bio/holt-run:latest otter --version
```

`holt-run` is rebuilt weekly and also whenever this repository publishes a
release. To build locally, pin a release for a reproducible image:

```bash
git submodule update --init holt
docker build --build-arg OTTER_VERSION=v1.3.0 -t holt-run ./holt/holt-run
```

`OTTER_VERSION` defaults to `latest`. Builds accept both `linux/amd64` and
`linux/arm64` (`docker buildx build --platform ...`). Each tool is checked
against the release's `checksums.txt` before it is installed, and the
environment YAML files come from the same release, so binaries and runtime
definitions cannot drift apart.

The image also contains SSH, JupyterLab with the Posit Ark kernel, and a
pre-initialized `otter-core` environment created with `enva`; see
[holt](https://github.com/otterlab-bio/holt) for ports and user management.


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

`methx` release binaries link HDF5 statically on both operating systems; installing
HDF5 separately is not required. For source builds, install CMake plus a C
compiler (e.g. `build-essential` on Linux or Xcode Command Line Tools on macOS),
then run `cargo build --locked --release --features download` inside `methx`.
The release build uses a native musl compiler on Linux to keep libc static too.

## Build and release checks

The **Build Toolchain Platforms** GitHub workflow builds `otter`, `otter-install`,
`craftmake`, `matsrun`, `pairbam`, `seq2mat`, `xenofilx`, `enva`, `fastqcx`, `methx`,
and `qctb` on native runners for all four platforms. It builds `bamdriver` as a
Go library; there is no `bamdriver` binary to download. Each component repository
also has a native build workflow. The umbrella release workflow publishes the
combined assets, checksums and manifest only after all four jobs pass. The checks
cover binary format, architecture, CLI startup, Linux static linkage and the
installer's offline plan; they do not run full scientific workflows or assert
third-party package availability on every operating system.


## Compatibility note

Some source-level symbols, generated state paths, or older release assets may still use `xdxtools`. That is a migration-era compatibility name; do not infer from a renamed output file that every code and runtime path has been renamed.

[Back to the manual](README.md) · [Documentation hub](../README.md)
