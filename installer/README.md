# otter-install

`otter-install` is a statically compiled Go installer for the **OTTER**
(**O**rchestrated **T**ranscriptomic, **T**umor-xenograft (PDX/CDX), and **E**pigenomic **R**eporting Workflow)
bioinformatics toolchain. It replaces the legacy `scripts/install.sh` with a
single self-contained binary that:

- Downloads pre-built static binaries from GitHub Releases
- Creates the required `otter-core` conda environment (and optional
  `otter-snakemake` / `otter-extra` environments)
- Verifies pinned tool versions (Bismark, Bowtie2)
- Runs the Craftmake `ReferenceBuild` workflow to download, build, and publish
  an immutable reference genome release

## Build

The installer uses only the Go standard library, so it builds as a fully
static binary with no external dependencies:

```bash
make build
# or
CGO_ENABLED=0 go build -trimpath -o otter-install .
```

## Usage

```bash
./otter-install [OPTIONS]
```

| Flag | Description |
| --- | --- |
| `--install-dir PATH` | Override binary installation directory (default `~/.cargo/bin`) |
| `--skip-envs` | Skip conda environment creation |
| `--non-interactive` | Use all defaults without prompting |
| `--dry-run` | Print all actions without executing |
| `--version VER` | Release version (e.g. `v0.3.0`); default `latest` |
| `--releases-repo REPO` | Primary GitHub release repo (owner/name) |
| `--fallback-releases-repo REPO` | Fallback GitHub release repo (owner/name) |
| `--github-proxy URL` | Optional GitHub proxy prefix |
| `--lang LANG` | Interface language: `en` or `zh` |
| `--reference-build` | Download and run the Craftmake ReferenceBuild workflow |

## Environment variables

- `GITHUB_TOKEN` / `GH_TOKEN` / `GITHUB_PAT` — token for private release downloads
- `GITHUB_RELEASES_REPO` / `OTTER_RELEASES_REPO` — primary release repo override
- `GITHUB_FALLBACK_RELEASES_REPO` / `OTTER_FALLBACK_RELEASES_REPO` — fallback override
- `GITHUB_PROXY_PREFIX` / `OTTER_GITHUB_PROXY` — GitHub proxy prefix
- `OTTER_INSTALL_LANG` — interface language override
- `OTTER_REFERENCE_BUILD_*` — ReferenceBuild field overrides (see below)

## ReferenceBuild

When `--reference-build` is set, the installer writes a
`reference-build.yaml` and invokes Craftmake to download, build, and publish an
immutable reference genome release into the registry.

Required overrides:

- `OTTER_REFERENCE_BUILD_FASTA_URL`
- `OTTER_REFERENCE_BUILD_GTF_URL`
- `OTTER_REFERENCE_BUILD_REFERENCE_ID`
- `OTTER_REFERENCE_BUILD_RELEASE`
- `OTTER_REFERENCE_BUILD_ORGANISM`
- `OTTER_REFERENCE_BUILD_ASSEMBLY`
- `OTTER_REFERENCE_BUILD_FASTA_CHECKSUM_VALUE`
- `OTTER_REFERENCE_BUILD_GTF_CHECKSUM_VALUE`

Optional overrides include `OTTER_REFERENCE_BUILD_BACKEND` (default `local`),
`OTTER_REFERENCE_BUILD_PARTITION`, `OTTER_REFERENCE_BUILD_REGISTRY_ROOT`, and
the tool binary paths (`OTTER_REFERENCE_BUILD_SAMTOOLS_BINARY`, etc.).
