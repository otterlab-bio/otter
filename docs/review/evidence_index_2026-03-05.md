# 证据索引（更新于 2026-03-05）

## A. 主仓职责落地证据

1. Runtime 初始化与嵌入资产
- `cmd/init.go`
- `internal/assets/assets.go`

2. Config 生成链路
- `cmd/create.go`
- `internal/config/config.go`

3. Snakemake 进程管理与环境 fallback
- `internal/workflow/snakemake.go`
- `internal/enva/enva.go`
- `cmd/run.go`

4. 动态作业调度
- `internal/workflow/manager.go`
- `internal/engine/local.go`
- `internal/engine/slurm_array.go`

5. 状态与恢复
- `internal/workflow/state.go`
- `internal/workflow/manager.go`

## B. 子仓职责与工具链证据

1. 规则侧实际命令依赖
- `inst/rules/*.smk`（命令集：`enva/fqc/xenofilter/paireads/htseq2matrix/methrix-cli/qctb/gomats`）

2. 安装侧覆盖
- `scripts/install.sh`（`TOOLS` 映射含上述命令）
- 兼容项：`methrix -> methrix-cli` 软链接

3. 构建侧覆盖
- `scripts/build-all-submodules.sh`（`required_bins` 与规则命令一致）

4. 一致性门禁
- `scripts/verify_toolchain_consistency.sh`
- `.github/workflows/go.yml`
- `.github/workflows/release.yml`

5. 发布回归证据门禁
- `scripts/capture_release_evidence.sh`
- `scripts/verify_release_evidence.sh`
- `docs/review/release_evidence_latest.md`
- `docs/review/release_evidence_latest.env`

## C. 可执行验证结果（2026-03-05）

1. 聚焦测试
- 命令：`conda run -n go-env go test ./internal/workflow ./internal/engine ./cmd -run "TestShouldPreflightRNAsplicing|TestLocalEngine|TestState|TestManager" -count=1`
- 结果：
  - `ok github.com/xdxtools/xdxtools-go/internal/workflow`
  - `ok github.com/xdxtools/xdxtools-go/internal/engine`
  - `ok github.com/xdxtools/xdxtools-go/cmd`

2. 一致性检查
- 命令：`bash scripts/verify_toolchain_consistency.sh`
- 结果：`Toolchain consistency check passed.`

3. 子仓状态快照
- 命令：`git submodule status`
- 结果：8 个子仓（`Paireads/enva/fastqc-rs/gomats/htseq2matrix-go/methrix-cli/qctb/xenofilter-go`）指针正常。
