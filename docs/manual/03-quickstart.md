# 3. Quick start

Otter authors canonical v1 projects, marked on disk by `project.lock.yaml`.
Projects created by earlier releases on the legacy compatibility layout
(marked by `.otter/assets.manifest.json`, with `config/otter.yaml` under
`userspace/<jobid>/`) are still detected: authoring commands refuse them, and
[migration](#migrating-a-legacy-project-to-v1) converts them into canonical
projects.

## Canonical v1 project

### 1. Initialize the project

```bash
otter init my_project
```

This pins the packaged workflow assets into `my_project/` and records their digests:

```text
my_project/
├── project.lock.yaml   # track marker and pinned asset digests
├── workflows/          # packaged workflow assets, including the Snakefiles
│   └── rules/          # Snakemake compatibility rules
├── environments/       # packaged environment declarations
├── schemas/            # packaged JSON schemas
└── runs/               # immutable run snapshots, written by config resolve
```

The rules live under `workflows/` rather than at the project root because a
Snakefile's `include:` directives resolve relative to the Snakefile itself.
Keeping them together is what lets a canonical project run without writing any
asset into the project root.

### 2. Provide a reference registry release

Canonical projects lock a logical reference id and release digest; they never copy genome files into the project. Build a release once, or select an existing one:

```bash
export OTTER_REFERENCE_ROOT=/shared/otter/references
otter reference build \
  --id hg38 --release GRCh38.p14 \
  --organism 'Homo sapiens' --assembly GRCh38 \
  --fasta /staging/genome.fa.gz --gtf /staging/genes.gtf.gz
```

See [reference migration](08-reference-migration.md) for the layout, manifest, and checksum contract.

### 3. Create the project intent

```bash
otter create \
  --output my_project \
  --fastq /data/fastq \
  --pdata /data/samples.csv \
  --mode RRBS \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --reference-primary hg38@GRCh38.p14
```

This verifies every declared reference against the registry and writes `project.yaml`, `samples.tsv`, and `references.lock.yaml`. `samples.tsv` records each FASTQ path relative to the project root, plus group and adapter columns; changing it later requires a new run.

For PDX scenarios, name the graft and host references instead:

```bash
otter create \
  --output my_project \
  --fastq /data/fastq \
  --mode RNASEQ \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --reference-graft hg38@GRCh38.p14 \
  --reference-host mm10@GRCm38.p6
```

### 4. Resolve an immutable run

```bash
otter config validate --config my_project/project.yaml --schema v1

otter config resolve \
  --project my_project/project.yaml \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --backend local
```

On a shared cluster, record the site once instead of repeating flags. `otter
site generate` probes the cluster and writes the profile; on a SLURM site it
needs the policy fields it cannot discover:

```bash
otter site generate --id production --backend slurm \
  --partition cpu112c --account genomics \
  --reference-root "$OTTER_REFERENCE_ROOT"
otter site validate production

otter config resolve --project my_project/project.yaml --site production --backend slurm
```

With `--site` and no `--reference-root`, the registry root comes from the profile.

Resolution prints the snapshot path. The snapshot is read-only and binds scenario, executor, backend, references, inputs, digests, and resources:

```text
my_project/runs/run-20260915T094657Z-eghubq/run.yaml
```

### 5. Dry-run, then run

```bash
otter run \
  --run-id run-20260915T094657Z-eghubq \
  --project-dir my_project \
  --executor craftmake \
  --phase step1 \
  --backend local \
  --dry-run \
  --foreground
```

`--dry-run` maps to `craftmake plan`: it resolves the workflow catalog and reports the task graph without executing anything. Drop `--dry-run` to execute, and swap `--backend local` for `--backend slurm` on a configured cluster.

`--run-id` is a convenience for `--config my_project/runs/<run-id>/run.yaml`. Otter never resolves `project.yaml` implicitly at run time; the snapshot must already exist.

### Running from any directory

You do not need to `cd` into the project. The snapshot is the execution boundary
and it records every path absolutely — the run root, work/results/logs/state
directories, the project config, the pinned workflow assets, the resolved
reference release, and each sample's FASTQ path with its checksum. Nothing is
discovered from the working directory, and `OTTER_REFERENCE_ROOT` is a build-time
input only, so a run does not read it.

```bash
# works from anywhere: the path printed by "otter build" is absolute
otter run \
  --config /data/projects/my_project/runs/run-20260915T094657Z-eghubq/run.yaml \
  --executor craftmake --phase step1 --backend local --foreground
```

Three inputs are still resolved relative to your shell, and only these:

| Input | Behaviour |
| --- | --- |
| `--config <path>` | Resolved against the working directory, so use an absolute path when running from elsewhere. |
| `--run-id <id>` | Expands to `<working-directory>/runs/<id>/run.yaml`, so it **does** assume you are in the project. Pass `--project-dir <dir>` instead, or `cd` first. |
| `--catalog <path>` | Resolved against the working directory. Omitting it is usually right: the snapshot already pins the catalog the run resolved against. |

The executors then place themselves correctly on their own: the Snakemake path
switches to the project directory internally, and the Craftmake worker is started
with the project directory as its working directory.

### Authoring in one step

Steps 1 to 4 are also available as a single command:

```bash
otter build \
  --project-root my_project \
  --fastq /data/fastq \
  --pdata /data/samples.csv \
  --mode RRBS \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --reference-primary hg38@GRCh38.p14 \
  --backend local
```

`otter build` chains init, create, validate, and resolve, then stops before
execution and prints the snapshot path. It produces exactly the project the four
steps produce; the e2e rehearsal asserts that artifact by artifact.

It defaults to the `craftmake` executor. Pass `--executor snakemake` to record
the Snakemake compatibility executor in the snapshot instead. The executor
belongs to the run snapshot, so the choice changes nothing in `project.yaml` and
can differ between runs.

A directory that already holds a `project.yaml` is refused rather than
overwritten. Authoring intent is not rewritten in place, because doing so would
silently invalidate every snapshot already resolved from it; use
`otter config resolve` to freeze a further run from the existing project.

## Migrating a legacy project to v1

Otter no longer authors legacy projects, but a project created by an earlier
release still carries `config/otter.yaml` under `userspace/<jobid>/`. That file
is **not accepted by any executor**: running it means converting it to a
canonical project.

Before migrating, the legacy file can be sanity-checked (`--schema auto` also
detects it), and an old checkout's pinned assets can still be verified:

```bash
otter config validate --config old_project/userspace/demo_rrbs/config/otter.yaml --schema legacy
otter assets verify --project old_project --strict
```

`otter config migrate` converts the configuration into canonical project intent.
Every reference must be named as `id@release`, because the legacy configuration
records filesystem paths rather than registry releases:

```bash
otter init migrated
otter config migrate \
  --input old_project/userspace/demo_rrbs/config/otter.yaml \
  --output migrated/project.yaml \
  --reference-root "$OTTER_REFERENCE_ROOT" \
  --reference-primary hg38@GRCh38.p14
```

Migration reports adopted fields, deprecated fields, and conflicts, and writes
nothing when a conflict remains.

It writes project intent only, so it needs a canonical project root (created by
`otter init` above) that already carries the pinned workflow assets the resolver
digests. Migrating into a bare directory leaves a project that cannot be resolved,
and the command warns when you do.

Resolve and run the migrated project — note that this selects the Snakemake
executor, because the legacy adapter maps the compatibility track onto it:

```bash
otter config validate --config migrated/project.yaml --schema v1

otter config resolve --project migrated/project.yaml \
  --reference-root "$OTTER_REFERENCE_ROOT" --backend local

otter run --config migrated/runs/<run-id>/run.yaml \
  --executor snakemake --dry-run --foreground
```

Executing this needs an `enva` or conda runtime providing Snakemake; `--dry-run`
validates configuration and workflow planning without one.

## Inspect and control tasks

```bash
otter task list
otter task status <task-id>
otter task logs <task-id> --follow
otter task stop <task-id>
otter status my_project
```

Without `--foreground`, Otter creates a background task and prints its task ID.

[Back to the manual](README.md) · [Next: analysis modes](04-analysis-modes.md)
