# 4. Analysis modes

Otter exposes three primary `create --mode` values and uses `--species2` to enable a PDX two-species workflow.

## RRBS

Reduced Representation Bisulfite Sequencing uses restriction-aware preparation and methylation quantification:

```bash
otter create \
  --fastq ./fastq \
  --mode RRBS \
  --pdata ./samples.xlsx \
  --species1 hg38
```

Typical outputs include trimmed FASTQ, aligned BAM/BAI, Bismark coverage, methylation matrices/HDF5, and QC reports.

## WGBS

Whole-Genome Bisulfite Sequencing uses the shared bisulfite workflow interface with different input scale and coverage expectations:

```bash
otter create \
  --fastq ./fastq \
  --mode WGBS \
  --pdata ./samples.xlsx \
  --species1 hg38
```

WGBS is documented as a supported workflow shape, but full production-scale qualification remains a separate evidence boundary.

## RNA-seq

RNA-seq uses STAR-compatible alignment, HTSeq-compatible counting, `seq2mat` matrix conversion, and optional `matsrun`/rMATS splicing:

```bash
otter create \
  --fastq ./fastq \
  --mode RNASEQ \
  --pdata ./samples.xlsx \
  --species1 hg38
```

The main data path is:

```text
FASTQ → fastqcx/Trim Galore → STAR → counts → seq2mat → matsrun/rMATS → qctb
```

## PDX

Add `--species2` to enable graft/host separation:

```bash
otter create \
  --fastq ./fastq \
  --mode RRBS \
  --pdata ./samples.xlsx \
  --species1 hg38 \
  --species2 mm10
```

The PDX path adds:

```text
dual-reference evidence → xenofilx → pairbam/bamdriver → downstream analysis
```

The same pattern applies to RNA-seq PDX when `--mode RNASEQ` is selected.

## Component mapping

| Stage | Main component |
| --- | --- |
| Raw FASTQ QC | `fastqcx` |
| Graft/host classification | `xenofilx` |
| Paired BAM filtering | `pairbam`, `bamdriver` |
| Expression matrix | `seq2mat` |
| Splicing | `matsrun`, external rMATS |
| Methylation/HDF5 | `methx`, external Methrix/R |
| QC aggregation | `qctb` |

[Back to the manual](README.md) · [Next: advanced usage](05-advanced-usage.md)
