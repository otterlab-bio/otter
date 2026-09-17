---
name: otter
description: "Operates, troubleshoots, develops, validates, and documents the otter bioinformatics workflow stack: the root CLI and independent craftmake, enva, fastqcx, xenofilx, pairbam, seq2mat, matsrun, qctb, methx, and bamdriver repositories. Use when creating or migrating projects, resolving immutable runs, running Local/SLURM workflows, diagnosing operator failures, changing repository contracts, testing releases, or maintaining user documentation."
---

# otter repository skill

Use this skill for code, documentation, release, and validation work in the `otter` root repository or one of its direct submodules.

## First pass

1. Identify the active repository from the working directory, top-level files, nearest README, and git remote.
2. Treat every submodule as an independent repository. A parent gitlink update is required when a submodule change is intended to affect the parent checkout.
3. Read the owning README, CLI entrypoint, package manifest, and relevant tests before changing a user-facing contract.
4. Keep current product names and external scientific standards distinct. See [repo-map.md](references/repo-map.md) for historical aliases.
5. For documentation work, read [documentation.md](references/documentation.md) before restructuring an entrypoint.
6. For an operational request, load [operations.md](references/operations.md). For a
   focused subtool, load [operator-contracts.md](references/operator-contracts.md).

## Product model

```text
otter → craftmake → enva → operators → bamdriver
```

- `otter` (**OTTER**: **O**rchestrated **T**ranscriptomic, **T**umor-xenograft (PDX/CDX), and **E**pigenomic **R**eporting Workflow) owns project initialization, input/configuration, run orchestration, task control, embedded workflow assets, and user documentation.
- `craftmake` owns native workflow compilation, Local/SLURM execution, SQLite state, recovery, cancellation, and reports.
- `enva` owns rattler-first environment lifecycle and explicit compatibility with conda, mamba, and micromamba.
- `fastqcx`, `xenofilx`, `pairbam`, `seq2mat`, `matsrun`, `qctb`, and `methx` are focused operator CLIs.
- `bamdriver` owns shared BAM/BGZF primitives used by BAM-consuming operators.

## Route the task before acting

| Request | Owning surface | First evidence |
| --- | --- | --- |
| Install or verify the stack | root `installer/` + `enva` | `otter-install -dry-run`, `enva validate --all` |
| Create/resolve/run a project | root `otter` | [operations.md](references/operations.md) |
| Compile, execute, resume, or inspect a DAG | `craftmake` | `skills/craftmake/SKILL.md` |
| Build or select a genome release | root `reference` commands / Craftmake ReferenceBuild | registry manifest + checksums |
| FASTQ/BAM/matrix/QC/methylation processing | owning operator | [operator-contracts.md](references/operator-contracts.md) |
| Existing `config/otter.yaml` | root migration adapter | validate → migrate → resolve; never execute it |
| Cross-repo release claim | all affected owners | real binary help, tests, e2e, and artifact verifier |

Do not start by editing the umbrella repo when a focused operator owns the
behavior. Do not make an operator responsible for orchestration that belongs to
Otter or Craftmake.

## Known-good operational path

For a user request that does not require source changes:

1. Verify `otter`, `enva`, and `craftmake`; verify Bismark/Bowtie2 through
   `enva run otter-core -- ...`.
2. Confirm FASTQ suffixes, pdata sample IDs, scenario, and reference roles.
3. `otter init`, `otter create`, `otter config validate`, then
   `otter config resolve` — or use `otter build`.
4. Plan **every published phase** with `--dry-run`; planning only `step1` is not
   evidence that downstream phases resolve.
5. Execute the immutable snapshot, monitor through `otter task`, and verify
   published artifacts.
6. When anything changes in project intent, references, inputs, executor,
   backend, or resources, resolve a new run instead of overriding the old one.

The exact commands and failure routing are in
[operations.md](references/operations.md).

## Authoring flow and legacy support

```text
otter init → otter create → otter config resolve → otter run
```

There is one authoring track: `otter init` and `otter create` produce a
**canonical v1** project (`project.lock.yaml`, `project.yaml`, `samples.tsv`,
`references.lock.yaml`). The legacy authoring track and its `--legacy` flag have
been removed; do not document them.

Legacy support is now detection plus migration only:

- A pre-existing legacy project (created by an earlier release) is recognised by
  its `.otter/assets.manifest.json` marker. `init`, `create`, and `build` refuse
  such a directory and route it at `otter config migrate`.
- `otter config migrate` converts a legacy `config/otter.yaml` into canonical
  project intent; `otter config validate --schema legacy` (or `auto`) is the
  pre-migration sanity check; `otter assets verify` still checks an old
  checkout's pinned assets.
- A directory carrying both markers is an error rather than a silent preference.

`otter build` chains the canonical steps — init, create, validate, resolve — and
stops before execution, printing the snapshot path. It is a shortcut over the
same functions the standalone commands call, not a second authoring path, and the
offline rehearsal asserts the two produce byte-identical authoring artifacts. It
defaults to the Craftmake executor; `--executor snakemake` records the Snakemake
compatibility executor in the snapshot instead. A directory that already holds a
`project.yaml` is refused rather than overwritten, because rewriting project
intent in place would silently invalidate every snapshot already resolved from
it; use `otter config resolve` to freeze a further run.

### Canonical project layout

```text
project/
├── project.lock.yaml     # track marker + pinned asset digests
├── workflows/            # packaged Snakefiles (pinned)
│   └── rules/            # Snakemake compatibility rules (pinned)
├── environments/  schemas/
└── runs/                 # one immutable run.yaml per resolved run
```

The rules live under `workflows/` rather than at the project root because a
Snakefile's `include:` directives resolve relative to **the Snakefile's own
directory**, not the working directory. Keeping the rules beside the Snakefiles
is what lets a canonical project run with nothing written into the project root.
Pre-existing legacy projects keep their Snakefiles at the project root, which is
why the executor's search order checks the current directory before
`workflows/`.

### What is fixed, and when

Four rules explain most of the "why was this refused" questions. Docs that violate
them look plausible and fail on first use, so check a change against all four:

1. **The snapshot is the only thing an executor consumes.** A raw legacy
   `config/otter.yaml` is refused by *both* executors with migration guidance. To
   run a pre-existing legacy project, `otter init` a canonical root,
   `otter config migrate` into it, `config resolve`, then run. Never document
   `otter run --config .../config/otter.yaml`.
2. **The backend and the resource envelope are fixed at resolve time.** They come
   from `project.yaml`'s `resources:` merged with the selected site profile
   (`mergeResources(project.Resources, siteResources)`), then validated against the
   target. At run time `--backend`/`--engine`, every `--stepN-*` and `--slurm-*`
   resource flag, and `--copy-fastq` are **refused**; only `--parallel-jobs`,
   `--load-ratio`, and `--workflow` remain settable. Resource units are `MiB`/`GiB`
   and `HH:MM:SS` under `resources.defaults` / `resources.phases.<phase>`.
3. **Editing `project.yaml` invalidates existing runs.** The snapshot pins the
   project digest, so a later `otter run` reports `run snapshot drift detected` and
   refuses. Resolve a new run after any intent change.
4. **PDX selection comes solely from the reference roles.** Naming both
   `--reference-graft` and `--reference-host` is what selects a PDX scenario,
   and the host and graft species come from the releases' own organism metadata.
   The legacy-only flags (`--legacy`, `--species1`/`--species2`,
   `--genome1-*`/`--genome2-*`, `--gtf1`/`--gtf2`, `--star-index1`/`-2`,
   `--conda-env`) no longer exist and are rejected as unknown flags. A `create`
   that names no reference role fails with
   `scenario "rrbs" requires --reference-primary as id@release`.

Use one example reference identity across the docs. The established pair is
`hg38@GRCh38.p14` and `mm10@GRCm38.p6` (with `hg19@GRCh37.p13-gencode-v19` for
migration examples); inventing a new one per chapter makes the manual look
inconsistent for no benefit. The release-label convention
`<assembly>-<annotation-source>-<annotation-version>` is guidance, not enforcement
— the schema only requires `^[A-Za-z0-9._-]+$`.

### A run does not depend on the working directory

The snapshot pins every path absolutely — run root, work/results/logs/state, the
project config, the pinned workflow assets, the resolved release, and each
sample's FASTQ with its checksum. So prefer suggesting the absolute snapshot path
`otter build` prints; it works from any directory, and the executors place
themselves correctly on their own (the Snakemake path `os.Chdir`s into the project
directory; the Craftmake worker is started with it as its working directory).
`OTTER_REFERENCE_ROOT` is a build-time input and is not read at run time.

Only three inputs resolve against the caller's shell, and they are the ones to get
wrong when writing an example:

- `--config` — relative to the working directory; use an absolute path.
- `--run-id` — expands under the working directory, so it *does* assume the caller
  is in the project. Pair it with `--project-dir` or `cd` first.
- `--catalog` — relative to the working directory when supplied. Omitting it is
  usually correct, because the snapshot already pins the catalog.

`otter site generate` writes a site profile from observed machine facts. It
discovers the toolchain, cluster name, partitions, and root candidates, but
never invents policy — partition, account, and QOS must be supplied. Prefer
`OTTER_SITE_PROFILE` or the default search path to select a profile rather than
overriding `HOME`, because `HOME` also steers `enva` environment discovery.

## Runtime dual-track

The runtime is dual-track. Existing production workflows use Snakemake
compatibility assets; Craftmake integration is still being validated. Do not
write documentation that implies complete Snakemake replacement unless the source
and accepted evidence explicitly support that claim.

## Troubleshooting protocol

Classify before changing code:

1. **Authoring failure** — inspect FASTQ pairing, pdata aliases, reference roles,
   registry verification, and project markers.
2. **Resolution failure** — inspect schema, site profile, workflow asset digest,
   absolute paths, reference manifest, and resource units.
3. **Planning failure** — identify the workflow/phase/catalog entry; compare the
   snapshot's selected executor with the requested executor.
4. **Execution failure** — read the task's `stderr.log`; controller summaries
   intentionally do not contain every tool diagnostic.
5. **Publication failure** — inspect declarations, artifact manifest, checksums,
   and transactional staging; never synthesize a success marker.
6. **Scientific parity failure** — separate orchestration success from
   comparator acceptance. A completed task graph is not scientific validation.

For Craftmake-specific diagnosis, use the troubleshooting section in
`skills/craftmake/SKILL.md`.

## Toolchain selection

- Go work: `conda activate go-env` for the root, `craftmake`, `pairbam`, `bamdriver`, `matsrun`, `seq2mat`, and `xenofilx`.
- Rust work: `conda activate rust_build` for `enva`, `fastqcx`, `methx`, and `qctb`.
- Load focused commands from [build-and-test.md](references/build-and-test.md) before building or testing.

## Installer

The release installer is a statically compiled Go binary in `installer/` (standard
library only, `CGO_ENABLED=0`). It downloads pre-built static binaries from GitHub
Releases, creates conda environments, verifies pinned tool versions, and can run the
Craftmake ReferenceBuild workflow. The binary is published to
[`otterlab-bio/otter`](https://github.com/otterlab-bio/otter).
The legacy `scripts/install.sh` remains for compatibility. When documenting install
steps, prefer the Go installer and keep the shell installer as the compatibility path.

## Documentation mode

When asked to redesign or beautify a README, use README mode: improve the whole information architecture, not only the decoration. Apply this order unless the repository has a stronger need:

```text
Value → Proof → Mechanism → First use → Detail
```

The first screen should answer what the repository is, who benefits, and where to go next. Prefer real CLI examples, output contracts, tests, benchmarks, and repository-native diagrams over generic decoration. Do not create a hero image merely to fill space; if a visual asset is useful, keep commands and essential instructions in Markdown.

For the root repository:

- Update `README.md` and `README_zh.md` together when user-facing setup or product language changes.
- Link current docs through `docs/README.md` and the task-oriented manual through `docs/manual/README.md`.
- `docs/` holds only the manual, `examples/`, `schema/`, and the hub `README.md`. Do not add a new top-level doc file without adding it to the hub index at the same time; an index entry that points at nothing is worse than no entry.
- Preserve visible limitations around WGBS, production scale, deferred matrices, and the Snakemake/Craftmake boundary.

For a submodule:

- Make its README independently useful: one-sentence value, input/output contract, install, minimal example, limits, tests, and repository link.
- Do not import root-only assumptions into a standalone operator README.
- Update root integration docs only when the submodule interface or workflow contract changes.

## Validation

Use the smallest relevant validation first:

1. Markdown links, command names, and paths match source.
2. README image references and SVG metadata pass the README audit when the audit script is available.
3. Go: `gofmt`, focused tests, `go test ./...`, and `go vet ./...` as appropriate.
4. Rust: `cargo fmt --all -- --check`, `cargo clippy --all-targets --all-features --locked -- -D warnings`, and `cargo test --all-targets --all-features --locked` when the repository workflow uses them.
5. Run the linter on edited files after substantive changes.

Operational work is accepted only when the documented command reaches the
requested boundary:

- authoring: expected `project.yaml`, `samples.tsv`, and
  `references.lock.yaml`;
- resolution: immutable `runs/<run-id>/run.yaml`;
- planning: non-empty tasks for every published phase;
- execution: state and logs tied to the phase-scoped run identity;
- publication: production verifier accepts the artifact manifest/checksums.

### Verify documented flags against the binary, not against memory

Documented flags drift silently, and a skill or tutorial that names a flag the
binary does not have costs the reader a failed run. Take flag names from the
built binary's own help before writing them:

```bash
go build -o /tmp/otter-check . && /tmp/otter-check run --help
go build -o /tmp/craftmake-check ./cmd/craftmake && /tmp/craftmake-check run --help
```

Distinguish flag families that look interchangeable but are not:
`craftmake report|cancel|status|logs` take `--state <state.sqlite>` plus `--run`,
while `craftmake run|plan|validate` take `--state-dir`. Likewise `--workers` and
`--max-parallel` are different flags on the same command. The cheapest reliable
check is to enumerate one column per flag across all subcommands:

```bash
for c in run plan validate resume report cancel status logs; do
  printf '%-9s state-dir=%s state=%s\n' "$c" \
    "$(/tmp/craftmake-check $c --help 2>&1 | grep -c -- '--state-dir')" \
    "$(/tmp/craftmake-check $c --help 2>&1 | grep -cE -- '--state ')"
done
```

### Run the offline e2e rehearsal for cross-command changes

A change that touches more than one of `init`, `create`, `config resolve`, or
`run` should be exercised through the rehearsal rather than by unit tests alone,
because the failure mode this project actually hit was command-to-command
disagreement rather than a broken command in isolation:

```bash
go build -o /tmp/otter . && (cd craftmake && go build -o /tmp/craftmake ./cmd/craftmake)
bash scripts/e2e/otter_e2e.sh --otter /tmp/otter --craftmake /tmp/craftmake
```

It is offline: it never downloads a genome and never runs an index builder. Two
legs are optional, so a stage count below the documented total is not itself a
failure:

- the `otter-install` leg runs only with `--installer <path>`;
- the `otter build` leg runs by default and is skipped with `--skip-build`.

The legacy *migration* leg always runs: it starts from a fixture
`config/otter.yaml` written by the script (legacy authoring is gone) and drives
validate → migrate → resolve → snakemake dry-run plus the executor pairing
matrix.

The build leg is a comparison, not a smoke test. It authors each scenario twice —
once through the manual chain, once through `otter build` — and fails if
`project.yaml`, `samples.tsv`, or `references.lock.yaml` differ. Run snapshots are
deliberately not compared, because each embeds its own run directory and
timestamp. Both paths must read the *same* input tree: `samples.tsv` records
inputs relative to the project root, so giving the two paths separate input
directories makes the manifest differ for a reason unrelated to the authoring
code.

Never claim a command was tested when only the documentation was edited. Never change a submodule and silently leave the parent gitlink stale.

## Safe operating rules

- Preserve unrelated work and start with read-only inspection.
- Do not rewrite historical evidence to apply current branding.
- Do not commit, push, tag, publish assets, or open a PR without explicit authorization.
- Do not add secrets, tokens, private paths, or undocumented local-machine assumptions to examples.
