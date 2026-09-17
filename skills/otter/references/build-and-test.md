# otter build and test matrix

## Root repo

### `otter`

- Toolchain: `conda activate go-env`
- Build: `go build -o otter .`
- Test: `go test -v ./...`
- Static checks: `go vet ./...`
- Release-style build: `./scripts/build.sh vX.Y.Z`

### Offline end-to-end rehearsal

Unit tests exercise the commands; the rehearsal exercises them *against each
other*, which is the failure mode this project actually hit (command-to-command
disagreement rather than a broken command in isolation). Build the binaries it
drives — `otter`, `craftmake`, `stub-registry`, and optionally `otter-install`:

```bash
mkdir -p build
go build -o build/otter .
( cd craftmake && go build -o ../build/craftmake ./cmd/craftmake )
go build -o build/stub-registry ./internal/e2esupport/cmd/stub-registry
( cd installer && go build -o ../build/otter-install . )

bash scripts/e2e/otter_e2e.sh \
  --otter build/otter \
  --craftmake build/craftmake \
  --stub-registry build/stub-registry \
  --installer build/otter-install
```

It is offline: it writes a simulated reference registry using the production
layout and verifier, downloads no genome, and runs no index builder. Each stage
records its command, stdout, stderr, and exit code under `--artifacts-dir`.

Expected totals depend on which optional legs run, so always record the flags
alongside the count. With every leg enabled the rehearsal reports **103 stages,
0 failures**; without `--installer` it reports **101**. Do not treat a lower count
as a regression without checking the flags first.

Useful flags: `--scenarios rrbs,rnaseq,bs-pdx,rna-pdx`, `--skip-legacy`,
`--skip-build`, `--keep` (preserve the work directory for inspection).

### `otter-install` (`installer/`) — separate Go module

`installer/` has its own `go.mod`, so the root `go test ./...` and `go vet ./...`
do **not** cover it. Build and test it from inside that directory:

- Toolchain: `conda activate go-env`
- Build: `cd installer && go build ./...`
- Test: `cd installer && go test ./...`
- Static checks: `cd installer && go vet ./...`

A root-level `go vet ./installer/...` fails with `directory prefix installer
does not contain main module or its selected dependencies`; that message means
the module boundary was crossed, not that the code is broken.

## Go submodules

### `craftmake`

- Toolchain: `conda activate go-env`
- Build: `go build -o craftmake ./cmd/craftmake`
- Test: `go test ./...`
- Notes: workflow catalog under `workflows/`; several tests read it, so run tests
  from the `craftmake` directory rather than from a bare worktree that has no
  submodule contents.

### `pairbam`

- Toolchain: `conda activate go-env`
- Build: `go build -o pairbam ./cmd/pairbam`
- Test: `go test ./...`

### `bamdriver`

- Toolchain: `conda activate go-env`
- Build: prefer consumer-driven validation; if needed use `go test ./...`
- Test: `go test ./...`

### `matsrun`

- Toolchain: `conda activate go-env`
- Build: `go build -o matsrun ./cmd/matsrun`
- Test: `go test ./...`

### `seq2mat`

- Toolchain: `conda activate go-env`
- Build: `go build -o seq2mat cmd/seq2mat/main.go`
- Test: `go test ./...`
- Notes: Gene mapping assets may be embedded; keep data-path assumptions aligned with the repo README.

### `xenofilx`

- Toolchain: `conda activate go-env`
- Build: `go build -o xenofilx ./cmd/xenofilx`
- Test: `go test ./...`

## Rust submodules

### `enva`

- Toolchain: `conda activate rust_build`
- Build: `cargo build --release`
- Test: `cargo test`

### `fastqcx`

- Toolchain: `conda activate rust_build`
- Build: `cargo build --release`
- Test: `cargo test`

### `methx`

- Toolchain: `conda activate rust_build`
- Build: `cargo build --release`
- Test: `cargo test`
- Notes: HDF5 development libraries may be required for a full build.

### `qctb`

- Toolchain: `conda activate rust_build`
- Build: `cargo build --release`
- Test: `cargo test`

## Validation strategy

- Prefer repo-local formatter and tests first.
- For root `otter` changes, run `go test -v ./...` and `go vet ./...` when the change can affect shared CLI or config behavior.
- Run the offline rehearsal when a change touches more than one of `init`, `create`, `config resolve`, `run`, or `build`, or when it changes the pinned asset layout. Per-command unit tests cannot see command-to-command disagreement.
- A change to `internal/assets/v1layout.go` must update the resolver digest list in `internal/config/resolver/resolver.go` in the same change. The two are one contract, and the rehearsal is what catches a mismatch.
- After changing the CLI, re-verify any flag or stage count you have documented against a real run. `otter build --help` and the rehearsal's summary line are the sources of truth.
- For shared library changes in `bamdriver`, validate at least one downstream consumer when feasible.
- If a command cannot run because a system dependency is missing, report the missing dependency instead of guessing success.
