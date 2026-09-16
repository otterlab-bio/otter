# Gate 6 最终交班：新旧工具链比较与 Methx 注释优化

> 更新时间：2026-08-26
>
> 本文是下一位 agent 的接续入口。完整比较报告见 `docs/gate6-toolchain-comparison-report.md`。

## 1. 已完成事项

### Gate 6 运行与比较

- 现代 Otter → Craftmake → Slurm 主线的多场景 canary 已获得真实执行证据。
- 历史 RRBS clean executor parity、recovery 和 semantic evidence 已接受。
- RNA-seq r31 Craftmake/Snakemake fresh pair 已完成全部 phase，`artifact verify` 和 `artifact compare` 通过。
- BS-PDX 与 RNA-PDX 已获得真实 Slurm PDX paired `step2-check` evidence；每个场景有三次 paired scheduler repeats，结论仅为 bounded/descriptive scheduler comparison。
- 人类 RRBS、三个 RNA-seq、RNA-PDX 的现代路径已完成相应分析/发布校验；具体 scope 和剩余 gate 见比较报告。

### BS-PDX 数据集切换

- 原主线 `SRR23802966` 因超大 WGBS 输入和 Xenofilx OOM incident 废弃。
- 新主线为 `SRR36187610`，71,242,412 paired reads，graft `hg38` + host `mm10`。
- 获取、解码、checksum、pair audit、acquisition provenance、step1、step2、step2-check、step3 均完成。
- 原始大数据 incident 证据保留，不要删除或重用为主验收结果。

### Methx 注释优化

源码子仓库：`methx/`

- 当前本地 `methx` 子仓库待提交 diff 主要覆盖内存 annotation interval index、sorted interval/prefix-max query、Rayon 并行处理和 FASTA/CpG extraction robustness。
- BS-PDX benchmark 使用的 v2 persistent binary index 与 `--annotation-index` 运行时 wiring 已在远端 staged/production binary 中验证，但尚未在当前本地 `methx` 工作树中看到对应源码改动；后续 release 必须把 source revision 与 binary 对齐。
- BS-PDX 全量 benchmark：7,290,833 CpGs，约 3m40s，exit 0，五类产物全部生成，annotation details 7,290,834 行，HDF5 shape `(1, 7290833)`。
- 正式 checker：controller `41687475`，child `41687489`，controller exit `0`；Methx child 约 1m55s。
- `assays.h5` 与 `methrix_data.h5` SHA-256：`eef88a4de5329de2772cf8f2dacc3fb4b3b64f0c6cb52372cf9d3e930e6d0444`。
- 正式 catalog 未修改；曾用一次性 runtime wrapper 通过 controller 的 `METHX` 显式补入 v2 index，避免 workflow digest drift。wrapper 不是科学产物，也不应复制进 catalog。

### QCTB / ENVA

- `qctb` 已支持 immutable `otter.run/v1` 和 `--config-dir`，从 `paths.work` 派生 canonical QC 路径并严格校验。
- `enva` 已将 Rust Bismark 3.1.0 和外部 pinned Bowtie2 作为环境契约，去除 legacy Perl Bismark/Bowtie2 Conda 依赖，并修正 Conda run separator 行为。

## 2. 本次文档入口

- `docs/gate6-toolchain-comparison-report.md`：正式新旧工具链比较报告。
- `docs/gate6-canary-matrix.md`：比较轴、场景矩阵和 2026-08-26 consolidated status。
- `docs/benchmark-plan.md`：benchmark 计划、解释边界和剩余 gate。
- `docs/active_context.md`：系统上下文中的最新 checkpoint。
- `docs/notes/2026-08-16-gate6-bs-pdx-craftmake-execution.md`：BS-PDX 历史执行 incident 与恢复记录。
- `docs/notes/2026-08-22-gate6-bs-pdx-SRR36187610-handoff.md`：数据集切换与 Xenofilx 处理记录。

## 3. 当前工作区提交范围

根仓库未提交项包括：

- `inst/rules/methrix_object.smk`
- `scripts/step3-check-bs-pdx-SRR36187610-controller.sh` 的 `--resume` 调整
- 新增 BS-PDX、Human RRBS、RNA-PDX publish controller
- 新增 RNA-PDX step3-check controller
- `enva`、`methx`、`qctb` 子模块指针变化
- 本次新增/更新的 Gate 6 报告、矩阵、benchmark、active context 和 handoff 文档

子仓库未提交项必须各自独立提交，然后根仓库再提交更新后的 gitlink。不要把子仓库文件直接从根仓库提交。

## 4. 验证和提交顺序

1. 根仓库脚本：对所有 `scripts/*controller.sh` 执行 `bash -n`。
2. `methx`：在 `rust_build` 中执行 `cargo fmt --all -- --check`、`cargo check --all-targets --all-features --locked`，并运行完整测试；interval-query/annotation 变更必须保留 regression tests。
3. `enva`：在 `rust_build` 中执行 formatter、check、test。
4. `qctb`：在 `rust_build` 中执行 formatter、check、test。
5. 每个子仓库单独检查 status/diff，分别提交 Conventional Commit。
6. 根仓库检查 gitlink、文档和 scripts diff，提交根仓库。
7. 提交后再次检查所有仓库 status，确认没有遗漏的未提交更改。

提交前不要修改 git config，不要 amend，不要 push；只有用户明确要求 push 时才 push。

## 5. 仍未关闭的 Gate

- 七输入 fresh modern-vs-legacy-equivalent 完整 scientific parity。
- representative `20 samples × 3 repeats` matrix。
- production-scale throughput 和 scheduler-pressure gate。
- WGBS `SRR6373947` primary-reference requalification 与 acquisition provenance。
- BS-PDX publish/完整 artifact manifest verification（若最终 promotion 需要）。
- 全流程真实 Snakemake PDX interruption/retry 与 scientific comparison。

不要把已通过的 Methx benchmark 重新跑成新的 binary index，也不要把实现性能结果写成整个现代工具链相对旧工具链的总 speedup。
