# Repository Guidelines

## CI Conventions

### Job summaries

Every workflow job ends with a single step named `Job summary` that writes a briefing to the
GitHub Actions summary page. The GitHub docs constrain the mechanism, and the convention follows
from those constraints:

- `GITHUB_STEP_SUMMARY` is unique and isolated **per step**, capped at 1 MiB per step, and only 20
  step summaries are displayed per job. A job therefore emits its whole briefing from one trailing
  step (`if: always()`), never by appending fragments from many steps.
- The step is always `continue-on-error: true`. A reporting helper must never be able to fail a
  passing job or mask a real failure. Assertions stay in the steps that can fail; the summary only
  describes what those steps produced, and reads the evidence files rather than restating claims.
- Render through `scripts/ci/job-summary.sh` so every repository reads the same way. Sections are
  fixed: verdict, run identity, what the job checked, contract table (expected vs observed),
  observed values, evidence artifacts, then **scope and limits** — an explicit statement of what the
  job does not claim. The limits section is mandatory in spirit: this project's gates are bounded
  contracts, and a summary that omits the bound is misleading.

The helper is duplicated into each repository because each is checked out independently and cannot
read a sibling at workflow runtime. `scripts/ci/check-job-summary-sync.sh` fails when a copy drifts
from the umbrella copy; umbrella `Go CI/CD` runs it.

## Project Structure & Module Organization
- Root Go CLI: `main.go`, `cmd/`, `internal/`, `pkg/`.
- Embedded workflow/runtime assets: `inst/` (Snakemake files, rules, env YAMLs, R helper scripts).
- Test fixtures: `testdata/` (FASTQ, pdata CSVs, e2e placeholders).
- Docs: `docs/` (architecture, build/install, manual pages).
- Helper scripts: `scripts/` (`setup.sh`, `build.sh`, `release.sh`, `build-all-submodules.sh`).
- Git submodules/tools live in their own directories (for example `enva/`, `fastqcx/`, `methx/`, `qctb/`, `xenofilx/`, `matsrun/`). Treat each as an independent module when changing code.

## Build, Test, and Development Commands
- Activate the correct toolchain first:
  - Go work: `conda activate go-env`
  - Rust/submodule work: `conda activate rust_build`
- `go build -o otter .` builds the main CLI locally.
- `go test -v ./...` runs all Go unit tests.
- `go vet ./...` runs static checks used by CI.
- `./scripts/build.sh vX.Y.Z` produces release-style binaries in `dist/`.
- `./scripts/setup.sh --dry-run` previews local install/runtime initialization.
- `./scripts/build-all-submodules.sh` builds companion binaries from submodules into `$HOME/.cargo/bin`.

## Development Environment
- Default Go environment: `go-env` (Conda).
- Default Rust environment: `rust_build` (Conda).
- Example session:
  - `conda activate go-env` for `go build`, `go test`, `go vet`.
  - `conda activate rust_build` for `cargo build` in Rust submodules (for example `enva/`, `fastqcx/`, `methx/`, `qctb/`).

## Coding Style & Naming Conventions
- Go style follows standard tooling: run `gofmt` (or `go fmt ./...`) before commit.
- Keep packages focused by domain (`internal/input`, `internal/engine`, `internal/workflow`, etc.).
- Use `snake_case` for workflow/config file names and `CamelCase` only where an external format requires it.
- Keep CLI flags and user-facing options explicit and stable (`--mode`, `--engine`, `--dry-run`, `--resume`).

## Testing Guidelines
- Go tests are in `*_test.go` files (mostly under `internal/`).
- Prefer table-driven tests for parser/validator logic.
- Use fixtures from `testdata/` instead of ad-hoc local files.
- For changes affecting execution modes, test both local and SLURM code paths where possible.
- Before opening a PR: run `go test -v ./... && go vet ./...`.

## Commit & Pull Request Guidelines
- Follow Conventional Commit style seen in history: `feat: ...`, `fix: ...`, `chore: ...`, `refactor: ...`.
- Keep commits scoped (one concern per commit), with imperative summaries.
- PRs should include:
  - What changed and why.
  - Affected modules/submodules.
  - Validation commands and key output.
  - Screenshots or terminal captures for TUI/output-format changes.
