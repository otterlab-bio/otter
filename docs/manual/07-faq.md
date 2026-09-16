# 7. FAQ

## Why is `otter` not found?

Check the installation directory and `PATH`:

```bash
command -v otter
printf '%s\n' "$PATH"
```

A source-built binary may still report compatibility-era `xdxtools` symbols. Check `./otter --help` before deploying it.

## Why does the installer return 404?

Verify the canonical repository and authenticate when assets are private:

```text
https://github.com/otterlab-bio/otter
```

Use `GITHUB_TOKEN`, `GH_TOKEN`, or `GITHUB_PAT` only through the installer's supported environment interface. Never commit tokens or put them in URLs.

## Why are submodules empty?

```bash
git submodule sync --recursive
git submodule update --init --recursive
git submodule status --recursive
```

## Can I replace Snakemake with Craftmake immediately?

No. The runtime is dual-track. Craftmake requires an immutable v1 run snapshot and is the native executor under integration; existing production workflows still use the explicit Snakemake compatibility path.

## Why did FASTQ pairing fail?

Check:

- R1/R2 suffixes match `--suffix1` and `--suffix2`.
- Filename-derived sample IDs match pdata `sampleid` exactly.
- Every sample has both mates.
- Duplicate sample IDs are removed.

## Why did SLURM submission fail?

Inspect site availability and the requested envelope:

```bash
sinfo
squeue -u "$(whoami)"
otter run \
  --config my_project/userspace/<jobid>/config/otter.yaml \
  --executor snakemake \
  --engine local \
  --dry-run \
  --foreground
```

Use the dry-run to distinguish a workflow/configuration problem from a scheduler submission problem. Compare partition, account/QOS, CPU, memory, and checker settings with your cluster policy.

## Why did `methx` report an HDF5 error?

Release binaries statically include the HDF5 runtime and should not need `HDF5_DIR` or `LD_LIBRARY_PATH`. Verify the binary first:

```bash
methx --version
file "$(command -v methx)"
ldd "$(command -v methx)" | grep -E 'hdf5|not found' || true
```

If the failure occurs during a source build, follow the native build instructions in [`methx/docs/BUILD.md`](../../methx/docs/BUILD.md). For native Methrix loading, convert the custom HDF5 output with `methx/scripts/export_methrix_hdf5.R`; do not call the custom file a native Methrix HDF5 directory.

## How do I inspect or resume a background run?

```bash
otter task list --all
otter task status <task-id>
otter task logs <task-id> --follow
otter run \
  --config my_project/userspace/<jobid>/config/otter.yaml \
  --executor snakemake \
  --engine local \
  --foreground
```

For a canonical v1 run, recover through the immutable snapshot and Craftmake controls described in the [execution contract](../execution-contract.md).

## Why do docs still mention FastQC, Bismark, Methrix, or rMATS?

Those are external tools, formats, standards, or scientific domains. They are not historical product names and should remain unchanged where they describe the actual compatibility contract.

[Back to the manual](README.md) · [Documentation hub](../README.md)
