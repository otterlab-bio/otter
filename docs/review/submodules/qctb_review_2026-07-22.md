# qctb remediation closure (2026-07-24)

## Status

The code-level blockers in this review are remediated in the current working tree. The historical R-compatible report requirement was intentionally replaced by the versioned native `qctb.report/1.0.0` contract, so no RDS or R-table parity is claimed.

## Closed findings

- C01/C02: main-repository nested configuration, workflow mode, graft, custom Qualimap directory, and `methrixh5` report paths are supported.
- C03/H08/M04: Excel and TSV share one typed schema definition with fixed columns, order, types, rounding, and `N/A`; Excel embeds a metadata sheet and TSV embeds schema/mode comments.
- H01/H02/H03/M05: strict numeric, duplicate-field, internal-consistency, SID, and Methrix contract validation is covered by tests.
- H04/H05/H06/H07/L01/L02/L04: `Cargo.lock` is tracked, binary fixtures are generated at runtime, publication is atomic, quoted command paths are tested, dead dependencies and the unsafe legacy Seqkit parser are removed, docs are updated, and Excel rejects integers outside its exact numeric range.
- Main workflow rules now declare Bismark PE reports, Qualimap `genome_results.txt`, STAR `Log.final.out`, and both Methrix workbooks as explicit producer/consumer artifacts.

## Remaining external validation

- Run real Snakemake RRBS/WGBS/RNA/PDX chains in an environment with Bismark, Qualimap, STAR, and Methrix installed.
- Run a real `methrix-cli -> qctb` workbook smoke test after the next submodule remediation.
- Rust MSRV policy remains a release-process decision; current local gates use the repository toolchain and locked dependencies.

---

# Original qctb system review (2026-07-22)

## 1. 审查结论

- **仓库基线**：`7ca0cc008c5a6a2d5c29ecaef8c99136d3e731cb`
- **分支**：`main`
- **审查状态**：只读审查完成，发现 3 Critical / 8 High / 7 Medium / 4 Low
- **发布结论**：**阻断发布**。主仓当前生成的 `config.yaml` 无法被 qctb 反序列化、Methrix 路径不一致导致指标静默丢失、Rust 输出不兼容 R QC 历史契约。

## 2. 门禁结果

| 命令 | 结果 |
|---|---|
| `cargo fmt --all -- --check` | 通过 |
| `cargo metadata --locked --no-deps` | 本机通过（依赖本地未跟踪 `Cargo.lock`） |
| `qctb --version` / `--help` | 通过 |
| 新鲜 `cargo clippy` | **未执行**（Ask 只读模式，禁止写 `target/`） |
| 新鲜 `cargo test` | **未执行**（同上） |
| `cargo audit` | **未安装** |

`Cargo.lock` 未跟踪（`.gitignore:7` 忽略），干净克隆无法复现依赖。发布 CI 无 `--locked`/`--frozen`。

## 3. 数据流

标准模式：`config.yaml` → `RawConfig` → `QCConfig` → 按 SIDs 顺序循环读 4 个 fqc、2 个 Trim Galore、Bismark PE、Qualimap、可选 Methrix → `Vec<QCSummary>` → Excel/TSV。声明了 `rayon` 依赖但实际是普通顺序循环（`aggregator.rs:211-231`），无 Rayon 使用点。

## 4. Critical 发现

### QCTB-C01：主仓当前生成的配置无法被 qctb 加载

- **位置**：`qctb/src/qc_summary/config.rs:40-42`、主仓 `cmd/create.go:560,783-788`、`inst/rules/bs_qc_summary.smk:22`、`inst/rules/rna_qc_summary.smk:21`。
- **证据**：主仓生成 `workflow.species.name` 为字符串数组（`speciesNames := []string{...}`）；qctb 声明同一字段为 `String`。当前所有新配置在 RRBS/WGBS/RNA-seq/PDX 反序列化阶段即失败。此外 qctb 实际需要 `workflow.species.graft`，但嵌套类型未声明 `graft`，错误地用 `name`（PDX 多物种数组）当 graft。
- **整改**：配置类型与主仓契约统一（`graft: String`、`name: Vec<String>`）；用 `cmd/create.go` 真实产物做集成测试。

### QCTB-C02：Methrix 实际报告路径错误，指标会静默丢失

- **位置**：`qctb/src/qc_summary/aggregator.rs:73-110,81-84,106`、`inst/rules/methrix_object.smk:67-69`。
- **证据**：主仓与 methrix-cli 写入 `<mcall>/methrixh5/CpG_*.xlsx`，qctb 查找 `<mcall>/CpG_*.xlsx`。找不到时不失败，返回 `None`，17 个 Methrix 字段被静默写成 `N/A`。`find_unique_sample_row` 缺样本也返回 `None` 不报错（`aggregator.rs:54-69`）。
- **整改**：拼接 `methylation_call/methrixh5`；报告存在但缺样本应报错而非降级 N/A。

### QCTB-C03：Rust 输出不兼容 R QC 历史契约

- **位置**：`qctb/src/qc_summary/excel.rs:19-61,218-242`、`inst/Rscripts/QC_summary.R:176-224`、`qctb/docs/requirements.md:33`。
- **证据**：要求明确写"Output must match R version exactly"，实际完全不匹配。R 标准约 49 列含 Trim/GC/bisulfite conversion/data volume 等；qctb 仅 25 列（加固定 Methrix 实际 42 列），缺 24 个历史基础字段。`trim_stats` 被解析存入聚合类型但 Excel/TSV 完全不写（`aggregator.rs:9`，编译诊断 `trim_stats is never read`）。RNA 模式 R 约 48 列，qctb 22 列且不读 Qualimap。R 用空格分隔，qctb 用 TSV；R 还生成 `qc_summary.RDS`，qctb 不生成；列名/舍入策略均不同。
- **整改**：确定唯一契约（R 兼容或带版本的新 schema）；逐字段恢复或迁移下游；RRBS/WGBS/RNA/PDX 各冻结 R/Rust golden diff。

## 5. High 发现

### QCTB-H01：FQC 数值转换可静默接受 NaN、Inf、负数和小数

- **位置**：`qctb/src/qc_summary/parsers/fqc.rs:61,79-85,100-112`。
- **证据**：先解析 `f64` 再 `as u64`/`as u32`，小数截断、负数/NaN 产生非预期整数、超范围饱和转换；多数据行后者覆盖前者；Q20/Q30 不校验 0–100；`bases_raw==0` 时 clean ratio 强制 0；`u64` 求和无溢出保护。

### QCTB-H02：Bismark/STAR/Qualimap 不检测重复字段或内部矛盾

- **位置**：`parsers/bismark.rs:23-58`、`parsers/rnaseq/star.rs:23-49`、`parsers/qualimap.rs:22-42`。
- **证据**：均用 `Regex::captures()` 只取首个匹配，重复字段/重跑残留段不报错；只检查分母非零，不检查 `aligned<=total`；百分比/Qualimap 字段保留为字符串不做数值校验；真实 Bismark fixture 仅 v0.24.2。

### QCTB-H03：样本集合未验证，空样和重复样可成功输出

- **位置**：`config.rs:113-117`、`aggregator.rs:211-231`。
- **证据**：顶层 `SIDs` 非空时无条件优先，不检查与嵌套列表一致；空列表生成只有表头的"成功"输出；重复 SID 生成重复行；SID 不 trim、不校验路径字符；Methrix 缺样本静默当可选缺失。

### QCTB-H04：`Cargo.lock` 未跟踪，发布依赖不可复现

- **位置**：`qctb/.gitignore:7`、`.github/workflows/release.yml:172-180`。
- **整改**：应用程序仓库应跟踪 `Cargo.lock`，CI 用 `--locked`/`--frozen`。

### QCTB-H05：三个 Methrix 测试依赖未跟踪的 Excel fixture

- **位置**：`qctb/.gitignore:21`、`parsers/methrix.rs:171-211`。
- **证据**：`testdata/methrix-qc-excel/*.xlsx` 被 `*.xlsx` 规则排除，干净克隆 `cargo test` 缺 fixture。文档"8/8 passing"过时（源码现有 24 个 `#[test]`）。

### QCTB-H06：TSV 和 Excel 输出不是原子写入

- **位置**：`main.rs:80-87,236-242`、`excel.rs:198-201,280-283`。
- **证据**：TSV 用 `File::create` 先截断旧文件，Excel 直接保存最终路径；失败丢失上一份完整报告；无 fsync/临时文件/原子 rename；不创建输出父目录。

### QCTB-H07：主仓调用未引用安全，带空格路径会失败

- **位置**：`inst/rules/bs_qc_summary.smk:22`、`inst/rules/rna_qc_summary.smk:21`。
- **证据**：`qctb --config {params.self_config}/config.yaml --output ...` 无引号，可参数拆分/命令失败/对不可信配置路径存在 shell 注入面。

### QCTB-H08：TSV 舍入造成精度丢失且与 R 不同

- **位置**：`main.rs:184-225,248-275`、`QC_summary.R:61,172-173`。
- **证据**：Q20/Q30/长度固定一位小数，ratio 固定四位小数；R 用有效数字或原值；`<0.00005` 的正值被写成 `0.0000`；Excel/TSV 同数据精度可能不同。

## 6. Medium 发现

- **QCTB-M01**：Qualimap 配置路径被忽略（主仓提供 `directories.qualimap` 但 qctb 不解析）。
- **QCTB-M02**：`--format` 任意非 `xlsx` 值静默选 TSV（`--format XLSX`/`json` 走 TSV，可能写入 `.xlsx`）。
- **QCTB-M03**：模式完全由 `--rnaseq` 决定，不读取配置 `mode`。
- **QCTB-M04**：Excel schema 依赖隐式默认值并混合单元格类型（sheet 未显式命名；同一列有数据写数字、无数据写 `"N/A"` 字符串）。
- **QCTB-M05**：Methrix 解析固定 sheet 名，输出缺列默认 0（`unwrap_or(0.0)` 无法区分真零与上游改名）。
- **QCTB-M06**：相同 Methrix 工作簿按样本重复完整读取（`O(samples×workbook size)`；无文件大小/样本数/内存上限；声明 rayon 但顺序执行）。
- **QCTB-M07**：Trim Galore 是强制输入但结果完全不输出（对未输出数据设强依赖，又不交付 Trim 指标）。

## 7. Low 发现

- **QCTB-L01**：未使用依赖（rayon/tracing/tracing-subscriber/thiserror）与 dead code 较多；`cargo clippy -- -D warnings` 预计失败。
- **QCTB-L02**：文档明显过期（active_context 称 8/8 tests 实为 24；称标准 25 列实为 42；称无已知问题）。
- **QCTB-L03**：MSRV 未被机器约束（无 `rust-version`，无 1.70 矩阵）。
- **QCTB-L04**：`u64` count 写 Excel 转 `f64`，超 `2^53` 失真。

## 8. 完成门禁缺口

- [ ] 主仓真实生成配置可被 qctb 加载。
- [ ] RRBS/WGBS/RNA/PDX golden output。
- [ ] 与 R `QC_summary.R` 逐字段差分。
- [ ] Methrix 路径修正与缺样本报错。
- [ ] NaN/Inf/负数/溢出/小数 count 测试；重复字段/千分位/异常空白测试。
- [ ] 空样/重复样/平铺嵌套冲突测试。
- [ ] Excel sheet/列类型/缺失值/格式 golden test。
- [ ] 输出原子性与失败保留旧文件测试。
- [ ] 路径含空格/特殊字符集成。
- [ ] 干净克隆可运行 fixture。
- [ ] 跟踪 `Cargo.lock`；CI 纳入 fmt/clippy/test/audit。
- [ ] Rust 1.70 MSRV 验证。
