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

`docs/` is deliberately small. It holds only:

- Root README: product value, stack, release boundary, minimal install/run path, and links.
- `docs/README.md`: the hub — what to read for a given need, and the maintenance policy.
- `docs/manual/`: the task-oriented tutorial for users, in reading order.
- `docs/examples/`: a complete canonical project, reference release, lock file, run snapshot, and samples manifest.
- `docs/schema/`: the JSON Schemas for configuration, snapshots, references, locks, and artifacts.
- `skills/otter/`: agent operating rules and validation matrix.

Earlier revisions of this repository carried architecture, operations, and
evidence documents directly under `docs/`. Those were removed to leave one
coherent user-facing surface, and their content now lives in the owning
submodule, in `AGENTS.md`/`CLAUDE.md`, or in dated records held outside `docs/`.
Two consequences follow:

- Do not reintroduce a top-level `docs/*.md` file without adding it to the hub
  index in the same change.
- A hub entry that names a document which does not exist is a defect, not a
  placeholder. The hub lists what is there.

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

A documented stage count can also drift, and two legs of
`scripts/e2e/otter_e2e.sh` are conditional: `otter-install` runs only with
`--installer`, while the `otter build` leg runs by default and is dropped with
`--skip-build`. State which flags produced a count whenever you record one, and
verify the count against a real run rather than adjusting the number to match
the last document.

## Remote host documentation

Remote operational docs should record the connection name and a read-only command
shape, and should not paste credentials, tokens, or private keys. When a remote
procedure is documented, state the working directory and the expected artifact
paths on the remote side, because those are the things a reader cannot infer from
local context.

Two operational traps are worth repeating wherever remote runs are documented:

- Overriding `HOME` breaks environment discovery. `enva` resolves named
  environments from `$HOME`-relative roots, so a scratch or sanitized `HOME`
  makes an environment that exists look missing. Prefer a site profile
  (`OTTER_SITE_PROFILE`) over a `HOME` override.
- A step failure is diagnosed from the task runtime directory, not the controller
  summary. The identifying stderr is under
  `<state>/runs/<run-id>/tasks/<id>/attempt-001/steps/*/stderr.log`.
