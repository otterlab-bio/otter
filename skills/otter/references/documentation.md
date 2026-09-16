# Documentation maintenance guide

## Information architecture

Use the sequence below for project homepages and focused tool READMEs:

```text
Value → Proof → Mechanism → First use → Detail
```

- **Value**: describe the user outcome in plain language.
- **Proof**: show a real output, tested capability, accepted evidence, or meaningful limitation.
- **Mechanism**: explain where the tool fits and what it consumes/produces.
- **First use**: provide one copyable path that can succeed end to end.
- **Detail**: link to advanced options, schemas, benchmarks, and development instructions.

Do not open with a long architecture explanation, a contributor guide, or a table of every flag. Keep limitations close to the claim they qualify.

## Root documentation layers

- Root README: product value, stack, release boundary, minimal install/run path, and links.
- `docs/README.md`: current documentation map and maintenance policy.
- `docs/manual/`: task-oriented tutorial for users.
- `docs/*.md`: current contracts, architecture, operations, release readiness, and evidence indexes.
- `docs/active_context.md`: concise current-state index; detailed experiment records belong in linked reports.
- `docs/review/`, `docs/archive/`, `docs/notes/`: historical evidence, retained without branding rewrites.
- `skills/otter/`: agent operating rules and validation matrix.

## Submodule README minimum

Every independent component README should answer:

1. What does this tool do?
2. What input does it accept?
3. What output does it write?
4. What is the shortest install path?
5. What is one real command example?
6. What is explicitly unsupported or compatibility-only?
7. How is it tested and where is the repository?

Keep package-specific details in the submodule. Explain root integration only in a short “Where it fits” section.

## Terminology

Use current names in new documentation:

- `otter` instead of `xdxtools`;
- `fastqcx` instead of `fastqc-rs`;
- `xenofilx` instead of `xenofilter-go`;
- `pairbam` instead of `Paireads`;
- `seq2mat` instead of `htseq2matrix-go`;
- `matsrun` instead of `gomats`;
- `methx` instead of `methrix-cli`;
- `bamdriver` instead of `bamdriver-go`.

Retain FastQC, MultiQC, FASTQ, Bismark, Methrix, HTSeq, rMATS, BAM, BGZF, HDF5, and Snakemake because they are external standards, tools, formats, or scientific concepts.

## Claim discipline

- Distinguish implemented, tested, accepted, bounded, deferred, and unsupported.
- Do not turn a benchmark into a general performance claim.
- Do not call a custom Methx HDF5 file a native Methrix HDF5 object; link to the explicit R exporter when native loading is needed.
- Do not describe Craftmake as the only production executor while Snakemake compatibility remains active.
- Do not use future or historical dates as evidence of current support without linking the source record.

## Verify commands before documenting them

Flags, subcommand names, and paths drift. A tutorial that names a flag the binary
does not have costs the reader a failed run, and the failure looks like an
environment problem rather than a documentation problem. Before writing a
command into a doc, confirm it against the owning binary:

```bash
go build -o /tmp/otter-check . && /tmp/otter-check <command> --help
(cd craftmake && go build -o /tmp/craftmake-check ./cmd/craftmake) && /tmp/craftmake-check <command> --help
```

Watch for near-miss flag pairs, which are the ones that survive review:

| Looks interchangeable | Actual split |
| --- | --- |
| `--state-dir` / `--state` | `run`, `plan`, `validate` take `--state-dir`; `status`, `logs`, `report`, `resume`, `cancel` take `--state <state.sqlite>` + `--run` |
| `--workers` / `--max-parallel` | Separate flags; `--workers` wins and `--max-parallel` is the zero-value fallback |

A documented stage count can also drift. `scripts/e2e/otter_e2e.sh` skips the
`otter-install` leg unless `--installer` is passed, so a rehearsal reporting fewer
stages than an older record is not necessarily a regression.

## Remote host documentation

Remote operational docs (for example `docs/gate6-paracloud-operations.md`) should
record the connection name and a read-only command shape, and should not paste
credentials, tokens, or private keys. When a remote procedure is documented,
state the working directory and the expected artifact paths on the remote side,
because those are the things a reader cannot infer from local context.

Two operational traps are worth repeating wherever remote runs are documented:

- Overriding `HOME` breaks environment discovery. `enva` resolves named
  environments from `$HOME`-relative roots, so a scratch or sanitized `HOME`
  makes an environment that exists look missing. Prefer a site profile
  (`OTTER_SITE_PROFILE`) over a `HOME` override.
- A step failure is diagnosed from the task runtime directory, not the controller
  summary. The identifying stderr is under
  `<state>/runs/<run-id>/tasks/<id>/attempt-001/steps/*/stderr.log`.
