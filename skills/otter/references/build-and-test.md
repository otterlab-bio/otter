# otter build and test matrix

## Root repo

### `otter`

- Toolchain: `conda activate go-env`
- Build: `go build -o otter .`
- Test: `go test -v ./...`
- Static checks: `go vet ./...`
- Release-style build: `./scripts/build.sh vX.Y.Z`

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
- For shared library changes in `bamdriver`, validate at least one downstream consumer when feasible.
- If a command cannot run because a system dependency is missing, report the missing dependency instead of guessing success.
