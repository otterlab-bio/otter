# 发布门禁清单（基线日期：2026-03-05）

## 目标
把“可发布”定义为一组可执行、可复核、可追溯的硬条件。

## Gate 1: 代码与测试
- [x] `conda run -n go-env go test ./internal/workflow ./internal/engine ./cmd -run "TestShouldPreflightRNAsplicing|TestLocalEngine|TestState|TestManager" -count=1`
- [x] `go.yml` 包含 `go test -v ./... && go vet ./...`

## Gate 2: 工具链一致性
- [x] `bash scripts/verify_toolchain_consistency.sh` 通过
- [x] `go.yml` 已执行一致性检查
- [x] `release.yml` 已执行一致性检查

## Gate 3: 子仓构建覆盖
- [x] `scripts/build-all-submodules.sh` 包含：`enva fqc xenofilter paireads htseq2matrix methrix-cli qctb gomats`
- [x] `scripts/install.sh` 的 `TOOLS` 覆盖同一命令集（含 `xdxtools`）

## Gate 4: 运行回归（待补证据）
- [ ] RRBS `run --dry-run`（local），证据由 `scripts/capture_release_evidence.sh` 生成并提交
- [ ] RNASEQ `run --dry-run`（slurm），证据由 `scripts/capture_release_evidence.sh` 生成并提交
- [ ] `bash scripts/verify_release_evidence.sh` 通过

## 发布结论
- 当前状态：**条件性可发布**（核心实现与门禁已到位，待补并提交最小 dry-run 回归记录）。
