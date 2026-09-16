# 2. Data preparation

A successful `otter create` needs a discoverable FASTQ pair for each sample and a pdata file whose sample IDs match those FASTQ names.

## FASTQ naming

The default pair suffixes are:

```text
sample1_R1.fastq.gz
sample1_R2.fastq.gz
```

Other suffixes can be supplied explicitly:

```bash
otter create \
  --fastq ./fastq \
  --mode RRBS \
  --suffix1 _1.fq.gz \
  --suffix2 _2.fq.gz
```

Sample matching is sensitive to the scanner's suffixes and the resulting sample name. Keep sample IDs consistent in filenames and pdata, including case.

## pdata

`--pdata` accepts Excel or CSV input through the root parser. The required column is `sampleid`. For grouping and downstream comparisons, use `sample_group` or `condition`.

Supported aliases include:

| Canonical field | Common aliases |
| --- | --- |
| `sampleid` | `样本编号`, `样本ID`, `sample_id` |
| `sample_group` | `样本分组`, `分组`, `group` |
| `condition` | `条件`, `treatment` |

Example CSV:

```csv
sampleid,sample_group,condition
sample1,control,control
sample2,control,control
sample3,treated,treated
sample4,treated,treated
```

`create` validates paired samples and, when supplied, validates the pdata relationship. Duplicate IDs and empty grouping values should be fixed before running.

## Project layout

A recommended input layout is:

```text
my_analysis/
├── fastq/
│   ├── sample1_R1.fastq.gz
│   ├── sample1_R2.fastq.gz
│   └── ...
└── samples.xlsx
```

`otter init my_project` creates the workflow asset layout. Keep raw data outside the project when possible and use absolute paths in controlled production environments.

## Preflight checklist

```bash
ls fastq/*_R1.fastq.gz
ls fastq/*_R2.fastq.gz
otter create \
  --fastq ./fastq \
  --mode RRBS \
  --pdata ./samples.xlsx \
  --output my_project/userspace \
  --jobid preflight_rrbs
```

If pairing fails, first compare the actual suffixes, sample-name case, and pdata `sampleid` values. Do not rename files blindly when they are referenced by another pipeline.

[Back to the manual](README.md) · [Next: quick start](03-quickstart.md)
