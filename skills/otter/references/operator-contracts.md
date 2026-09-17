# operator CLI contracts

These are focused tools, not alternative workflow controllers. Otter/Craftmake
owns orchestration; operators own one input/output contract.

## `fastqcx`

```bash
fastqcx --fastq sample.fastq.gz --summary qc/sample > reports/sample.html
```

- Inputs: `.fastq`, `.fastq.gz`, `.fq`, `.fq.gz`; quality bytes must fall within the supported Phred+33 range. The parser cannot infer a conceptual encoding from overlapping byte ranges.
- Outputs: HTML on stdout unless `--no-html`; FastQC-compatible
  `fastqc_data.txt` under `--summary`.
- Limits: whole-file totals are complete; per-position charts track the first
  10,000 positions by default. `--kmer` accepts 1–7.

## `xenofilx`

```bash
xenofilx run --graft graft.bam --host host.bam --output filtered \
  --threads 4 --sort-memory 2G
```

- Applies the exclusive threshold to each graft mate first. With host
  alignments present, either graft mate at or above the threshold discards the
  fragment; surviving fragments compare graft and host mate-score sums, with
  the lower total winning and ties discarded.
- `--recalculate-nm` requires `--graft-ref` and `--host-ref`.
- `--bisulfite` handles CT and GA conversion contexts.
- `--output-names/-w` supplies one explicit output name per graft input.
- BAM/FASTA primitives come from the shared `bamdriver` library.

## `pairbam`

```bash
# One merged BAM: keep one unique primary R1/R2 group.
pairbam --coord-sort input.bam output.bam

# Two BAMs: keep names present exactly once in both.
pairbam --coord-sort R1.bam R2.bam output_prefix
```

Dual mode fails on an ambiguous shared name rather than writing it to the
filtered-name manifest. Both modes publish BAM, optional BAI, and a
`*_filtered_readnames.txt` audit file.

## `seq2mat`

```bash
seq2mat --htseq_dir ./htseq --output_dir ./matrix
```

Writes count/normalized TSV matrices plus a versioned manifest. Release and
normal source builds embed mapping CSVs; R is needed only to regenerate those
CSVs from upstream `.rda` files.

## `matsrun`

```bash
matsrun run --root ./bam --pdata samples.xlsx \
  --seqlengthQC ./qc --gtf hg38.gtf --threads 10
```

- Requires `rmats.py` on `PATH`.
- Generates every species × pairwise-group contrast.
- Publishes five rMATS result families and `manifest.json` transactionally.
- `temp/` is staging-only and is absent from successful published output.

## `qctb`

```bash
qctb --config-dir runs/<run-id> --output qc_summary.xlsx
qctb --config legacy-config.yaml --output qc_summary.tsv --format tsv
```

- `--config-dir` means a canonical directory containing immutable `run.yaml`,
  not an arbitrary YAML directory.
- Output schema is `qctb.report/1.0.0`, not byte-identical legacy R output.
- Optional reports may be absent; a present report with invalid, duplicate, or
  missing required fields fails closed.
- Pass `--rnaseq` only when the configuration declares RNA-seq or RNA-PDX; it
  is required for those modes and rejected for other explicit modes.

## `methx`

```bash
methx process --input coverage_dir --output results --genome genome.ron
```

The custom `assays.h5` schema stores `/beta`, `/cov`, versioned metadata, and
1-based closed coordinates. It is directly readable with `rhdf5`, but is not a
native `methrix::load_HDF5_methrix()` directory. Convert with:

```r
source("methx/scripts/export_methrix_hdf5.R")
export_methx_h5_to_methrix(
  methx_h5_path = "results/assays.h5",
  output_directory = "results/methrix_h5",
  validate = TRUE
)
m <- methrix::load_HDF5_methrix("results/methrix_h5")
```

Do not use the historical `docs/r_scripts/` `assay001/assay002` loaders.

## `enva`

```bash
enva create --core
enva validate --all
enva run otter-core -- bismark --version
```

Bismark 3.1.0 and Bowtie2 2.5.5 are installed inside `otter-core`. Use
`enva adopt` before mutating an externally created conda/mamba/micromamba
prefix. Duplicate names fail until `--prefix` disambiguates them.

## `bamdriver`

This is a Go library, not a normal end-user workflow CLI. Validate changes with
`go test ./...` and at least one consumer (`pairbam` or `xenofilx`).
`cmd/bamroundtrip` and `cmd/nmoracle` are diagnostic/evidence tools; read their
`--help` before use.

