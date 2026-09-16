# methrix-cli 系统审查报告（2026-07-22）

## 1. 审查结论

- **仓库基线**：`c35185a58d0a58e5633d7c727dac2ba11065fec6`
- **分支**：`main`
- **原始审查**：2026-07-22 只读审查发现 2 Critical / 6 High / 6 Medium / 多 Low。
- **整改状态（2026-07-24）**：子仓代码整改完成，Rust 门禁和真实最小 CLI integration test 通过。HDF5 契约已收敛为版本化 custom schema，不再声称标准 HDF5Array/methrix loader 兼容。
- **发布结论**：代码、门禁与父仓发布编排阻断项已关闭；正式发布前仍必须由账户所有者完成 Git 凭据吊销/轮换。该账户操作不应由源码整改冒充完成。

## 1.1 整改关闭摘要（2026-07-24）

| 原发现 | 状态 | 关闭证据 |
|---|---|---|
| METHRIX-C01 Git 凭据暴露 | 外部未完成 | 不在源码中修改凭据；必须由账户所有者吊销/轮换并清理 remote URL |
| METHRIX-C02 loader 契约不成立 | 已关闭（契约收敛） | 固定 `methrix-cli.custom-hdf5/1.0.0`；metadata 明示仅支持 `rhdf5` direct access；README、设计、R smoke test 同步 |
| METHRIX-H01 父仓 release 绕过测试 | 已关闭 | `build-and-release` 明确 `needs: test`；test job 执行 methrix fmt、strict clippy 和 all-target/all-feature tests |
| METHRIX-H02 download feature 未进入正式构建 | 已关闭 | 子仓 E2E、父仓 release 和 `build-all-submodules.sh` 均以 `--features download` 构建 methrix |
| METHRIX-H03 非事务发布 | 已关闭 | `AtomicOutputSet` 覆盖 HDF5 alias、QC、annotation；测试覆盖 staging failure、发布中途 rollback 和 stale removal |
| METHRIX-H04 峰值内存 | 风险显著降低 | 样本结果直接写最终 columns；并发临时向量受线程数约束；stats 单 pass；HDF5 bounded chunk writing；Bismark 单样本记录仍按 worker 整体读入 |
| METHRIX-H05 Excel 行上限 | 已关闭 | workbook 仅保留 `ChIPseeker_By_Sample`；逐 CpG 明细迁移到 `CpG_annotation_details.tsv.gz` |
| METHRIX-H06 下载完整性 | 已关闭 | UCSC 固定 URL/MD5、流式 checksum、大小/超时限制、FASTA provenance、cache tamper validation、事务发布 |
| METHRIX-M01 线程上限 | 已关闭到处理/过滤范围 | sample processing 与 filter 在 bounded Rayon pool 内；stats 已顺序单 pass |
| METHRIX-M02 schema 漂移 | 已关闭 | schema name/version、`u32` coverage 和 loader compatibility 已冻结并由 native validator 检查 |
| METHRIX-M03 Bismark end 忽略 | 已关闭 | 要求单碱基 `end == start`，含 negative test |
| METHRIX-M04 GTF fallback 不确定 | 已关闭 | 精确 species GTF 优先；fallback 仅接受唯一候选，多候选列出路径并失败 |
| METHRIX-M05 side effects 未声明 | 已关闭 | 父仓 Snakemake 声明 `assays.h5`、alias、QC、annotation summary 和 details |
| METHRIX-M06 门禁缺口 | 已关闭到可用环境范围 | malformed HDF5、单/多线程等价、真实 CLI integration、事务测试和 CI R `rhdf5` smoke 已加入 |

## 2. 门禁结果

| 命令 | 结果 |
|---|---|
| `cargo fmt --all -- --check` | 通过 |
| `cargo check --all-targets --all-features --locked` | 通过 |
| `cargo clippy --all-targets --all-features --locked -- -D warnings` | 通过 |
| `cargo test --all-targets --all-features --locked -- --test-threads=1` | 通过（52 unit + 1 real CLI integration；0 failed / ignored / filtered） |
| `cargo build --all-targets --all-features --locked` | 通过 |
| 子仓及父仓 `git diff --check` | 通过 |
| 父仓 `go test -count=1 ./internal/assets` | 通过 |
| workflow YAML parse / build script `bash -n` | 通过 |
| 本机 R | R 4.6.0 可用，`rhdf5` 不可用 |

本机无法执行 R direct-schema smoke，因为 `requireNamespace("rhdf5")` 返回 `FALSE`。子仓 E2E workflow 已显式安装 `r-bioc-rhdf5`，通过真实 `methrix process` 产物运行 `tests/integration/test_r_compatibility.R`；标准 HDF5Array/methrix loader 不属于当前 custom schema 的支持契约。

## 3. 架构与坐标契约

处理链路：RON 参考或 FASTA 提取 CpG → 扫描排序 Bismark cov → 建立 `(contig, 0-based start) → CpG index` → 按样本并行读入全文件 → 对齐参考 → 全局 `Array2<f32>` beta 与 `Array2<u32>` coverage → 可选过滤未覆盖 → 输出。

坐标契约内部自洽：FASTA CpG 为 0-based end-exclusive；Bismark start 1-based 减一；HDF5/Excel 输出恢复 1-based closed。

Coverage 已从文档承诺的 `u16` 改为 `u32`，70,000 coverage 回归测试通过，不再截断。

## 4. Critical 发现

### METHRIX-C01：Git remote URL 包含明文 GitHub 凭据

- **范围**：Git 元数据，非受跟踪源码行。
- **影响**：仓库读写权限可能泄露，凭据可能进入日志与历史。
- **整改**：立即吊销/轮换凭据，改用无凭据 HTTPS 或 SSH，排查暴露范围。
- **跨仓观察**：此问题同样被 `htseq2matrix-go` 审查独立确认。

### METHRIX-C02：当前输出不满足标准 `loadHDF5SummarizedExperiment()` 契约

- **位置**：`src/hdf5/se_compat.rs:15-40`、`src/cli/process.rs:296-309`。
- **证据**：只写自定义 HDF5 根目录 dataset 与属性，不生成标准 `saveHDF5SummarizedExperiment()` 目录契约所需的 `se.rds` 等 R 端序列化对象；R 测试仅用 `rhdf5::h5read()` 手工读取后内存重建，未调用真实 loader；CI 只装 `r-bioc-rhdf5`。
- **影响**：只能证明“R 能用 rhdf5 读取自定义 schema”，不能证明标准 SummarizedExperiment/methrix loader 能直接加载，与项目承诺不符。
- **整改**：若承诺标准兼容，生成真实可加载目录与 `se.rds`；否则移除误导性兼容声明并提供正式 R loader。

## 5. High 发现

### METHRIX-H01：主仓 release 可绕过测试任务直接发布

- **位置**：`.github/workflows/release.yml:20-49`。
- **证据**：`build-and-release` 没有 `needs: test`；`test` job 不运行 cargo/R 门禁；主仓普通 CI 与 release test 均不测试 methrix。
- **整改**：给 `build-and-release` 加 `needs: test`，在当前子模块指针上运行 Rust 与真实 R 消费者门禁。

### METHRIX-H02：正式发布版 `download-genome` 默认不可用

- **位置**：`Cargo.toml:66-68`、`src/main.rs:174-189`、`.github/workflows/release.yml:186-191`、`scripts/build-all-submodules.sh:129-139`。
- **证据**：下载在可选 feature 中，默认 feature 为空；release 与构建脚本均不带 `--features download`。
- **整改**：正式构建启用 `download` 或从 CLI 隐藏。

### METHRIX-H03：核心产物非事务性发布

- **位置**：`src/hdf5/se_compat.rs:21`、`src/cli/process.rs:301-308`、`src/qc/report.rs:43`、`src/annotation/mod.rs:219`、`src/genome/cpg.rs:210-216`。
- **证据**：HDF5、Excel、RON 与 annotation 直接 `File::create` 最终路径；`assays.h5` 与兼容副本 `methrix_data.h5` 分两步；中途失败留截断或混合版本文件。
- **整改**：同目录临时文件写入，flush/sync 后原子 rename。

### METHRIX-H04：并行设计导致多份峰值内存

- **位置**：`src/cli/process.rs:59-60,62-78,110-115`、`src/processing/filter.rs:47-59`、`src/hdf5/se_compat.rs:68-82`。
- **证据**：主矩阵加 per-sample 结果加过滤矩阵加 HDF5 转置复制；2,800 万 CpG × 100 样本仅 beta+coverage 即约 44.8 GB，过滤/转置期接近 67 GB。
- **整改**：流式 Bismark 解析；分块 HDF5 写入；不要同时保留全部 `ProcessedSample`。

### METHRIX-H05：默认 annotation 报告超过 Excel 行上限

- **位置**：`src/annotation/mod.rs:169-217,225-251`、`src/main.rs:52-54`、`inst/rules/methrix_object.smk:76-84`。
- **证据**：annotation 默认开启，主仓调用不带 `--skip-annotation`；WGBS 超过 1,048,575 CpG 上限会在 HDF5/QC 已发布后失败。
- **整改**：明细改 TSV/Parquet/HDF5 或分 sheet，Excel 只保留汇总。

### METHRIX-H06：下载缺少完整性验证与原子安装

- **位置**：`src/genome/download.rs:39-74`。
- **证据**：直接 `reqwest::blocking::get`、直接写最终路径、无校验和、无 `.part`、无 `sync_all`、无大小/超时上限、无截断回归测试。
- **整改**：固定 URL、校验和、`.part` 原子安装、超时与大小上限。

## 6. Medium 发现

- **METHRIX-M01**：`--threads` 非端到端线程上限（自定义 pool 仅包围样本处理，后续过滤/stats 用全局 Rayon pool）。
- **METHRIX-M02**：coverage 从 `u16` 改 `u32` 但 schema 未版本化，消费者契约可能漂移。
- **METHRIX-M03**：Bismark `end` 只检查不早于 start，匹配时完全忽略，可能掩盖错误输入。
- **METHRIX-M04**：主仓规则找不到精确 GTF 时静默选目录中首个 GTF。
- **METHRIX-M05**：主仓未声明全部 side effect，`methrix_data.h5` 不受 Snakemake 生命周期管理。
- **METHRIX-M06**：兼容性与异常输入门禁不完整（缺真实 loader、多线程等价、截断 gzip、空文件、内存预算、R 差分；`tests/integration/test_full_pipeline.rs` 未被引入）。

## 7. Low 发现

`canonical_contig_name` 不处理 `Chr1`；标准染色体排除 `chrM/MT`；FASTA contig 长度用 `as u32` 可能截断；未固定 Rust toolchain；release `*-static` 未做链接验证；`find_bismark_files` 未先确认 regular file。

## 8. 主仓调用

- 已对齐：主仓调用 `methrix-cli`；Snakemake 声明 `assays.h5`、`methrix_data.h5`、QC、annotation summary 和 details；GTF fallback 只接受唯一候选；父仓 release 依赖 test 并执行 methrix Rust 门禁；release 与本地 submodule build 均启用 `download` feature。
- 已记录限制：本机缺少 `rhdf5`，R direct-schema smoke 只能由已配置依赖安装的子仓 E2E 执行；release 产物的实际静态链接属性仍应由现有 artifact/linkage verification 持续检查。

## 9. 完成门禁

- [ ] Git remote 凭据已由账户所有者吊销/轮换并清理暴露范围（外部任务）。
- [x] HDF5 契约收敛为 versioned custom schema，并明确仅支持 `rhdf5` direct access。
- [x] `build-and-release` 依赖 test，且父仓 release test 执行 methrix Rust 门禁。
- [x] HDF5、alias、QC 与 annotation 事务发布并覆盖 rollback/stale removal。
- [x] 样本并发和 HDF5 写入临时内存受界；不再创建完整转置副本。
- [x] annotation 明细迁移到 gzip TSV，不依赖 Excel 行上限。
- [x] genome 下载固定 release、URL、checksum、大小限制、provenance 与 cache validation。
- [x] 真实 CLI integration、多线程等价、malformed HDF5 与事务门禁已执行。
- [x] 子仓 E2E 配置真实 R `rhdf5` direct-schema smoke；本机因缺包未重复执行。

剩余已记录限制：每个活跃 worker 的 `BismarkReader` 仍会读入一个完整样本记录向量；若需要进一步压低极端 WGBS 峰值，应在后续版本改为逐行解析并直接填充最终 column。
