---
name: otter
description: "Use when working in the otter bioinformatics workflow repositories: the root CLI or its independent submodules craftmake, enva, fastqcx, xenofilx, pairbam, seq2mat, matsrun, qctb, methx, and bamdriver. Covers repository boundaries, toolchain selection, documentation maintenance, CLI-contract verification, and evidence-aware release language."
---

# otter repository skill

Use this skill for code, documentation, release, and validation work in the `otter` root repository or one of its direct submodules.

## First pass

1. Identify the active repository from the working directory, top-level files, nearest README, and git remote.
2. Treat every submodule as an independent repository. A parent gitlink update is required when a submodule change is intended to affect the parent checkout.
3. Read the owning README, CLI entrypoint, package manifest, and relevant tests before changing a user-facing contract.
4. Keep current product names and external scientific standards distinct. See [repo-map.md](references/repo-map.md) for historical aliases.
5. For documentation work, read [documentation.md](references/documentation.md) before restructuring an entrypoint.

## Product model

```text
otter → craftmake → enva → operators → bamdriver
```

- `otter` (**OTTER**: **O**rchestrated **T**ranscriptomic, **T**umor-xenograft (PDX/CDX), and **E**pigenomic **R**eporting Workflow) owns project initialization, input/configuration, run orchestration, task control, embedded workflow assets, and user documentation.
- `craftmake` owns native workflow compilation, Local/SLURM execution, SQLite state, recovery, cancellation, and reports.
- `enva` owns rattler-first environment lifecycle and explicit compatibility with conda, mamba, and micromamba.
- `fastqcx`, `xenofilx`, `pairbam`, `seq2mat`, `matsrun`, `qctb`, and `methx` are focused operator CLIs.
- `bamdriver` owns shared BAM/BGZF primitives used by BAM-consuming operators.

## Authoring flow and the two tracks

```text
otter init → otter create → otter config resolve → otter run
```

Both tracks author through the same commands, and the track is chosen by a flag
that must be consistent:

- Default: `otter init` and `otter create` produce a **canonical v1** project
  (`project.lock.yaml`, `project.yaml`, `samples.tsv`, `references.lock.yaml`).
- `--legacy` on **both** commands produces the **legacy compatibility** project
  (`config/otter.yaml` plus `.otter/assets.manifest.json`). Only when both pass
  `--legacy` is that loop promised; mixed use is refused with guidance.
- A project carrying both markers is an error rather than a silent preference.

`otter create` resolves its declared references against the registry
(`--reference-root`, `--reference-primary`, or `--reference-graft`/
`--reference-host` for PDX) and writes `references.lock.yaml`, so the result is
immediately resolvable. `otter config migrate` converts a legacy project to v1
intent, but migration produces project intent only: it needs a canonical project
root that already carries the pinned workflow assets the resolver digests.

The executor is **not** chosen by the track. It is fixed in the immutable
`runs/<run-id>/run.yaml`, and `--executor` must agree with the snapshot:
a canonical snapshot selects Craftmake, and a migrated legacy snapshot selects
Snakemake. A raw legacy `otter.yaml` is an authoring format that neither
executor accepts until it is migrated and resolved.

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
- Keep `docs/archive/`, `docs/review/`, and dated `docs/notes/` as historical evidence unless a specific correction is requested.
- Preserve visible limitations around Gate 6, WGBS, production scale, deferred matrices, and the Snakemake/Craftmake boundary.

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

It is offline: it never downloads a genome and never runs an index builder. It
does skip the `otter-install` leg unless you also pass `--installer <path>`, so a
stage count below the documented total is expected without one.

Never claim a command was tested when only the documentation was edited. Never change a submodule and silently leave the parent gitlink stale.

## Safe operating rules

- Preserve unrelated work and start with read-only inspection.
- Do not rewrite historical evidence to apply current branding.
- Do not commit, push, tag, publish assets, or open a PR without explicit authorization.
- Do not add secrets, tokens, private paths, or undocumented local-machine assumptions to examples.
