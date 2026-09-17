# 4. Analysis modes

`otter create --mode` selects the scenario. On the canonical track the **references
carry the species**: a release records its own organism and assembly, so you name
a release rather than a species.

| `--mode` | Scenario | Reference roles |
| --- | --- | --- |
| `RRBS` | `rrbs`, or `bs-pdx` with graft+host | `--reference-primary`, or `--reference-graft` + `--reference-host` |
| `WGBS` | `wgbs` (no PDX variant) | `--reference-primary` |
| `RNASEQ` | `rnaseq`, or `rna-pdx` with graft+host | `--reference-primary`, or `--reference-graft` + `--reference-host` |

`BSSEQ` is rejected: it does not distinguish RRBS from WGBS, so name one of them.

Each example below assumes you ran `otter init <dir>` first, which pins the
workflow assets the canonical track requires.

## RRBS

Reduced Representation Bisulfite Sequencing uses restriction-aware preparation and
methylation quantification:

```bash
otter create \
  --output my_project \
  --fastq ./fastq \
  --pdata ./samples.csv \
  --mode RRBS \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --reference-primary hg38@GRCh38.p14
```

Typical outputs include trimmed FASTQ, aligned BAM/BAI, Bismark coverage,
methylation matrices/HDF5, and QC reports.

## WGBS

Whole-Genome Bisulfite Sequencing shares the bisulfite workflow interface with a
different input scale and coverage expectation:

```bash
otter create \
  --output my_project \
  --fastq ./fastq \
  --pdata ./samples.csv \
  --mode WGBS \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --reference-primary hg38@GRCh38.p14
```

WGBS has no PDX scenario, so `--reference-graft`/`--reference-host` are refused
with that reason rather than silently ignored.

WGBS is documented as a supported workflow shape, but full production-scale
qualification remains a separate evidence boundary.

## RNA-seq

RNA-seq uses STAR-compatible alignment, HTSeq-compatible counting, `seq2mat`
matrix conversion, and optional `matsrun`/rMATS splicing. It needs an annotation,
which is part of the release:

```bash
otter create \
  --output my_project \
  --fastq ./fastq \
  --pdata ./samples.csv \
  --mode RNASEQ \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --reference-primary hg38@GRCh38.p14
```

The main data path is:

```text
FASTQ → fastqcx/Trim Galore → STAR → counts → seq2mat → matsrun/rMATS → qctb
```

## PDX

Graft and host are two **roles**, so PDX is named by declaring both rather than by
naming two species:

```bash
otter create \
  --output my_project \
  --fastq ./fastq \
  --pdata ./samples.csv \
  --mode RRBS \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --reference-graft hg38@GRCh38.p14 \
  --reference-host mm10@GRCm38.p6
```

Both roles are required together; supplying only one is refused. The same pattern
applies to `--mode RNASEQ` for RNA-PDX. Declaring the roles is what selects the
PDX scenario — there is no separate switch to set.

The project records the roles rather than a single primary:

```yaml
references:
  species:
    - role: graft
      selection: hg38@GRCh38.p14
    - role: host
      selection: mm10@GRCm38.p6
```

The PDX path adds:

```text
dual-reference evidence → xenofilx → pairbam/bamdriver → downstream analysis
```

### The legacy track differs here

With `--legacy` the species *are* flags, because that track predates the registry:

```bash
otter create --legacy \
  --fastq ./fastq --mode RRBS --pdata ./samples.xlsx \
  --species1 hg38 --species2 mm10 \
  --output my_project/userspace --jobid demo_rrbs
```

`--species1` and `--species2` are **legacy-track only**. On the canonical track the
first is ignored, and `--species2` merely signals PDX intent — the references must
still come from `--reference-graft`/`--reference-host`. A canonical `create` with
`--species1` and no reference flag fails with
`scenario "rrbs" requires --reference-primary as id@release`.

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
