# Otter installation

Otter (**OTTER: Orchestrated Transcriptomic, Tumor-xenograft (PDX/CDX), and Epigenomic Reporting Workflow**) is installed as a root CLI plus independent workflow components. Installation is currently dual-track:

- **Snakemake compatibility:** optional for established projects generated around `otter.yaml`.
- **Canonical v1/Craftmake:** the default path for new projects and immutable `run.yaml` execution.

Installing Craftmake does not remove or replace Snakemake automatically. Install the
compatibility environment only when an existing legacy project needs it.

## Prerequisites

- Linux or macOS; Linux with SLURM is the primary production target.
- Git with recursive submodule support.
- Go 1.24 or newer for the root CLI and Go components.
- `methx` is distributed as a release binary with its HDF5 dependency statically linked; HDF5 runtime environment variables are not required.
- Building `methx` from source still requires the Rust toolchain and the native build tools used by `hdf5-metno`.
- Paired FASTQ files and a matching samples manifest.

## Release installation

The release installer is a statically compiled Go binary published to
[`otterlab-bio/otter`](https://github.com/otterlab-bio/otter):

```bash
curl -fsSL -o otter-install https://github.com/otterlab-bio/otter/releases/latest/download/otter-install-linux-amd64-static
chmod +x otter-install
./otter-install
otter --help
```

The legacy shell installer remains available for compatibility:

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/otterlab-bio/otter/main/scripts/install.sh)
otter --help
```

Verify the release and its bundled assets before using it in production. If private assets are supported by the selected installer, provide credentials through the environment; never put a token in a committed URL or configuration file.

## Source installation

```bash
git clone --recurse-submodules https://github.com/otterlab-bio/otter.git
cd otter
conda activate go-env
go build -trimpath -o otter .
install -m 755 otter "$HOME/.cargo/bin/otter"
otter --help
```

For an existing checkout:

```bash
git submodule sync --recursive
git submodule update --init --recursive
```

The current submodule directories are:

```text
craftmake enva fastqcx xenofilx pairbam seq2mat matsrun qctb methx bamdriver
```

Use [build.md](build.md) and [submodules-build-guide.md](submodules-build-guide.md) for component builds.

## Runtime environments

Create the required core environment with Enva:

```bash
enva create --core
```

The Snakemake compatibility and extra analysis environments are optional:

```bash
enva create --core --snakemake
enva create --core --extra
enva list --detailed
enva validate --all
```

Rust Bismark 3.1.0 and Bowtie2 2.5.5 are provided together by the `otter-core` environment. The installer no longer downloads Bismark or Bowtie2 separately from GitHub releases; run related commands through `enva run otter-core -- ...`.

| Environment | Role |
| --- | --- |
| `otter-core` | Required dependencies and modern operators |
| `otter-snakemake` | Optional Snakemake compatibility path |
| `otter-extra` | Optional analysis and visualization dependencies |

Environment names are part of the runtime contract. If a checkout still emits historical `xdxtools-*` names, inspect its assets and lock files rather than manually renaming directories.

## Verify the installation

```bash
enva run otter-core -- bismark --version
enva run otter-core -- bowtie2 --version
enva run otter-core -- bismark_genome_preparation --help
```

The required versions are pinned in the core specification as `bismark=3.1.0` and `bowtie2=2.5.5`. The installer verifies both version reports after creating `otter-core`. Bioconda provides this Bismark build for Linux x86_64, Linux aarch64, and macOS arm64; macOS x86_64 is not supported by the recipe.

`bamdriver` is primarily a shared Go package and may not expose a standalone executable in every checkout. Some operators expose `--help` but not `--version`.

## First compatibility dry-run

Use this path for an established project layout:

```bash
otter init my_project
otter create \
  --fastq /data/fastq \
  --mode RRBS \
  --pdata /data/samples.csv \
  --output my_project/userspace \
  --jobid demo_rrbs

otter run \
  --config my_project/userspace/demo_rrbs/config/otter.yaml \
  --executor snakemake \
  --engine local \
  --dry-run \
  --foreground
```

The compatibility path consumes the generated `otter.yaml` and uses Snakemake explicitly.

## First canonical validation

For a new reproducible project, validate and resolve before execution:

```bash
otter config validate --config project.yaml --schema v1
otter config resolve --project project.yaml --backend local
otter run \
  --config runs/<run-id>/run.yaml \
  --executor craftmake \
  --phase step1 \
  --backend local \
  --foreground
```

The snapshot fixes inputs, references, executor, backend, resources, workflow assets, and digests. Runtime flags cannot silently mutate it.

## Troubleshooting

- **Empty submodules:** run `git submodule update --init --recursive`.
- **Snakemake unavailable:** validate `otter-snakemake`; Craftmake is not a blanket fallback for legacy workflows.
- **SLURM partially available:** use `otter site validate`; production resolution fails closed rather than silently selecting Local.
- **Historical product prefix:** inspect the environment assets and repository revision before deployment.
- **Bootstrap 404:** verify the canonical repository path and whether the selected release assets are private.

## Uninstallation

Remove only binaries and environments you own:

```bash
rm -f "$HOME/.cargo/bin/otter" "$HOME/.cargo/bin/craftmake" "$HOME/.cargo/bin/enva"
```

Use `enva remove` for managed `otter-*` environments. Do not recursively delete a project, reference registry, or shared environment root without confirming ownership.

[Back to the documentation hub](README.md)
