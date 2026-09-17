# 6. Component reference

Otter is the orchestration layer; the focused repositories own most specialized computation.

## `craftmake`

Native workflow compiler and Local/SLURM executor for immutable `otter.run/v1` snapshots. It owns DAG compilation, SQLite state, caching, recovery, cancellation, and reports.

```bash
craftmake doctor --backend local
craftmake plan --config run.yaml --phase step1 --catalog workflows/
```

Craftmake is under integration. Snakemake remains the explicit compatibility executor for existing production workflows.

## `enva`

Rattler-first environment lifecycle manager:

```bash
enva create --core
enva create --core --snakemake  # optional legacy compatibility
enva create --core --extra      # optional analysis dependencies
enva list --detailed
enva run otter-core -- <command>
enva validate --all
```

Bismark 3.1.0 (Rust suite) and Bowtie2 2.5.5 are provided by the `otter-core`
environment; the installer verifies the pinned versions after environment
creation. The maintained methylation rules invoke them through
`enva run otter-core -- ...` rather than any user-global installation.

External conda/mamba/micromamba environments require explicit compatibility handling or adoption.

## `fastqcx`

FASTQ QC reporter that writes HTML to stdout and FastQC-compatible summary files for MultiQC and `qctb`:

```bash
fastqcx --fastq sample.fastq.gz --summary qc/sample > reports/sample.html
```

## `xenofilx`

Graft/host classifier for PDX BAM inputs, including reference-based NM recalculation and bisulfite-aware scoring:

```bash
xenofilx run --graft graft.bam --host host.bam --output filtered
```

## `pairbam` and `bamdriver`

`pairbam` filters paired-end BAM records. `bamdriver` provides the shared BAM/BGZF, sort, index, FASTA, and alignment helpers:

```bash
pairbam --coord-sort input_R1.bam input_R2.bam filtered
```

## `seq2mat`

Converts HTSeq count files into TSV count/normalized matrices and a versioned manifest:

```bash
seq2mat --htseq_dir ./htseq --output_dir ./matrix
```

## `matsrun`

Generates and runs every pairwise rMATS contrast from grouped BAM files, pdata, SeqKit statistics, and GTF annotations:

```bash
matsrun run --root ./bam --pdata samples.xlsx \
  --seqlengthQC ./qc --gtf hg38.gtf
```

## `qctb`

Aggregates workflow QC into Excel or TSV from legacy configuration or immutable run snapshots:

```bash
qctb --config-dir runs/<run-id> --output qc_summary.xlsx
```

## `methx`

Converts Bismark coverage to custom HDF5 plus QC/annotation outputs. Use the standalone R exporter when native `methrix::load_HDF5_methrix()` loading is required:

```r
source("methx/scripts/export_methrix_hdf5.R")
```

[Back to the manual](README.md) · [Next: FAQ](07-faq.md)
