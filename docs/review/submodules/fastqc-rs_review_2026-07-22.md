# fastqc-rs 系统审查报告（2026-07-22）

## 1. 审查结论

- **原始审查基线**：`14ccb7b`（分支 `library-latest`）。
- **整改状态**：2026-07-25 本地 closure 完成；原始 1 Critical / 4 High / 8 Medium / 7 Low 均已修复或形成明确兼容边界，当前 Critical / High / Medium 为 0。
- **本地发布结论**：代码、异常输入、资源预算、真实消费者和主仓调用契约门禁通过；不再因 malformed/truncated FASTQ、HTML 网络访问或无界 k-mer/position 统计产生虚假成功、panic 或无界增长。
- **交付状态**：当前改动尚未 commit、push，也未更新主仓 submodule pointer。GitHub Actions 中固定外部工具下载链仍需远端实际运行后才能形成发布证据。

## 2. 门禁结果

| 门禁 | 结果 |
|---|---|
| `cargo fmt --all -- --check` | 通过 |
| `cargo check --all-targets --all-features --locked` | 通过 |
| `cargo clippy --all-targets --all-features --locked -- -D warnings` | 通过 |
| `cargo test --all-targets --all-features --locked` | 通过：23 unit + 4 binary integration，doctest 不再访问不存在的 fixture |
| `cargo build --all-targets --all-features --locked` | 通过 |
| `git diff --check` | 通过 |
| 100,000 reads × 150 bp，`k=7`，`--no-html` | 通过：28.847 秒，峰值 RSS 14,208 KiB；预算 120 秒 / 512 MiB |
| Babraham FastQC 0.12.1 | 真实 fixture 核心统计对照通过 |
| SeqKit 2.13.0 | 19 列模块中约定核心字段逐字段对照通过 |
| MultiQC 1.35 | 真实解析通过，发现 1 个 FastQC report |
| qctb parser | 定向单测通过；真实 `fqc` run4 产物消费通过 |
| 主仓 FastQC Snakemake contract | 两个规则路径参数已使用 `{value:q}`，静态契约测试通过 |
| GitHub Actions YAML | `yaml.safe_load` 解析通过 |

工具链：rustc/cargo 1.97.1，edition 2018，包 `fastqc-rs 0.3.4`，二进制 `fqc`，needletail 0.5.1。

## 3. 整改架构与发现 closure

当前数据流只有一个 FASTQ 解析事实来源：

```text
CLI / library
    -> KmerLength + AnalysisLimits validation
    -> analyze_with_options()
    -> FastqAnalysisResult
    -> HTML / fastqc_data.txt rendering
```

- `analyze()` 使用默认资源边界；`process()` 只渲染分析结果，不再维护第二套 parser loop。
- needletail 的 malformed record、截断 gzip、缺 quality、空 read、长度不一致和非法 Phred+33 均立即返回结构化错误。
- summary 目录只在完整分析成功后创建，失败时不发布部分报告。
- k-mer 限制为 1..=7；per-position histogram 最多保留 10,000 位；单 read 最长 10,000,000 bases；distinct read lengths 最多 100,000。
- HTML 生成删除 reqwest/CDN 抓取路径，不再发起网络请求；输出保留固定外链，浏览器完全离线打开时图表资源可能不可用。
- FastQC Basic Statistics 与 SeqKit GC 分母分别按各自语义计算；Q20/Q30 为碱基级，AvgQual 使用平均错误概率。

| 原始发现 | Closure |
|---|---|
| FASTQC-C01 | IUPAC/其他非 ACGT 字节归入 N 类，不再 panic；回归测试覆盖 |
| FASTQC-H01 | Q20/Q30 改为碱基级；AvgQual 对齐 SeqKit 2.13；真实差分通过 |
| FASTQC-H02 | sequence content 使用 A-T 与 G-C；测试通过 |
| FASTQC-H03 | 导出使用确定性排序；MultiQC 真实消费通过 |
| FASTQC-H04 | 删除运行时网络请求和 `unwrap()`；不可达代理 integration test 通过 |
| FASTQC-H05 | typed k-mer/read/position/distinct-length limits + 大输入机器预算通过 |
| FASTQC-M01..M08 | 空 histogram、malformed/truncated、CRLF、空 read、长/变长 read、doctest、FastQC/MultiQC 字段与百分位语义均已关闭 |
| FASTQC-L01..L07 | URL 拼写、重复 parser、文档、百分位命名、截断均值等已整改；浏览器端 CDN availability 保留为明确非阻断边界 |

以下各节保留原始审查 finding、触发条件和影响说明；其中代码行号对应原始基线，不代表整改后位置。

## 4. Critical 发现

### FASTQC-C01：非法碱基触发 panic，解析直接崩溃

- **位置**：`src/process.rs:140-149`（analyze）、`:343-352`（process，同逻辑）。
- **触发**：FASTQ sequence 含 `A/C/G/T/N`（含大小写）以外字节（IUPAC `R/Y/S/W/K/M/B/D/H/V`、`U`、`*`、`-`、空白等）。
- **证据**：`_ => panic!("Invalid base")`。
- **影响**：含 IUPAC 模糊碱基的 FASTQ（参考基因组导出、contig、UMI、变体调用输出）让 `fqc` 整个进程 panic，无优雅退出；库 API `analyze` 同样 panic 而非 `Result`。
- **整改**：返回 `io::Error`/自定义 error，或将非标准碱基并入 N 桶，至少 `panic!` 改 `return Err`。

## 5. High 发现

### FASTQC-H01：Q20/Q30/AvgQual 公式语义错误（按 read 平均而非按碱基）

- **位置**：`src/process.rs:619-642`（summary 计算）。
- **证据**：`mean_read_qualities` 是 `HashMap<mean_quality_of_read, count_of_reads>`；`q20_count` 统计 **mean quality ≥ 20 的 read 数**除以 read 总数得 "Q20%"；`avg_qual = Σ(q*c)/Σc` 是 reads 的平均 mean quality。与行业惯例（FastQC/seqkit 碱基级）、MultiQC 预期不符。
- **级联影响**：qctb `parsers/fqc.rs` 把 `fqc` 的 `Q20(%)/Q30(%)` 当碱基级使用，对长 read（150bp）严重高估；**直接污染 qctb 上报的 Q20/Q30**。
- **整改**：遍历 `base_quality_count[pos][score]`（已按碱基的 94-bin 直方图）聚合，按碱基计 Q20/Q30/AvgQual。

### FASTQC-H02：Per base sequence content 警告比较对象错误（T vs G 而非 T vs A）

- **位置**：`src/process.rs:399-404`。
- **证据**：`tg_diff = |bases[T] - bases[G]| / sum`；判定 `gc_diff(G-C)` 和 `tg_diff(T-G)` 任一 ≥0.20 fail / ≥0.10 warn。FastQC 原始模块比较 **A-T 和 G-C**（Chargaff 平衡），这里写成 **T-G** 与 **G-C**。
- **影响**：会误报/漏报 sequence content 警告，与 Babraham FastQC 不一致。
- **整改**：改 `at_diff = |A-T|/sum` 与 `gc_diff = |G-C|/sum`。

### FASTQC-H03：HashMap 输出顺序不确定，fastqc_data.txt 非确定性

- **位置**：`src/process.rs:370-405`、`fastqc_summary.txt.tera`（`base_count`/`read_lengths`/`gc_per_base` 用 `{% for pos, base_map in ... %}`）。
- **证据**：`rustc-hash::FxHashMap` 故意不保序；Per base sequence content / Sequence Length Distribution / Per base GC content 行顺序在不同运行间不可复现。
- **影响**：人工比对/回归测试困难；MultiQC 对部分模块期望按位置升序。
- **整改**：导出前对 key `usize` 排序后输出（`BTreeMap` 或 `collect`+`sort`）。

### FASTQC-H04：embed_source 同步阻塞拉取 6 个外部 CDN，无超时/重试/失败处理

- **位置**：`src/process.rs:717-724`（`embed_source` filter）+ `report.html.tera` head（bootstrap CSS、jquery、popper、bootstrap JS、vega、vega-lite、vega-embed 共 6 个 URL）。
- **证据**：`reqwest::blocking::get(&url).unwrap().text().unwrap()` 两次 unwrap，无 timeout/重试/缓存。
- **影响**：离线/受限网络/CDN 故障时 panic；reqwest 0.11 默认无连接超时可能挂起数十秒。主仓 snakemake 规则全用 `--no-html` 规避，但交互式使用仍危险。
- **整改**：设 `Client::builder().timeout(Duration::from_secs(10))`，失败回退外链 `<script src=url>` 或内嵌资源到二进制。

### FASTQC-H05：计数与内存无上限，k-mer map 与 per-position 直方图可能 OOM

- **位置**：`src/process.rs`（analyze/process）。
- **触发**：超大 FASTQ 或可变长 read（Nanopore 长度可达 10^5+）；`base_count`/`base_quality_count` key 为位置随最大 read 长度膨胀；`kmers` 在 k 大时（`u8` 允许到 255）指数爆炸。
- **影响**：无流式/采样策略（Babraham FastQC 前 200000 read 采样），大文件 RSS 可能数 GB。
- **整改**：k-mer 限长（≤7）、read 长度截断/采样上限、或文档化内存预算。

## 6. Medium 发现

- **FASTQC-M01**：`quartiles` 函数 `assert!(sum != 0)`，空向量 panic 而非错误（`process.rs:188`）。
- **FASTQC-M02**：截断 gzip/malformed FASTQ 仅置 `broken_read=true` 不返回错误也不停止；`fastqc_data.txt` 无字段反映；截断 gzip 可能产生部分 record 后才报错（`process.rs:155-157,333-335`）。
- **FASTQC-M03**：`gc_percentage` 整数除法丢精度（`gc_bases * 100 / called_bases`，`process.rs:104`）。
- **FASTQC-M04**：`phred33_score` 仅支持 Phred+33 范围 0..=93，对 +64/Solexa 静默拒绝，无自动检测（`process.rs:111-123`）。
- **FASTQC-M05**：CRLF/空 read 处理：0 长度 read 不报错但静默计入 `read_lengths[0]`，拉低 `avg_read_length`。
- **FASTQC-M06**：超长/可变长 read 无 capped binning（Vega-Lite 图表 X 轴爆炸）。
- **FASTQC-M07**：doctest 失败（`src/lib.rs:8-11` 用不存在 `sample.fastq`，`cargo test --all-features` exit 101）。
- **FASTQC-M08**：`fastqc_data.txt` 缺标准 FastQC 模块/字段，MultiQC 兼容性有限；自定义 `>>Seqkit Statistics` MultiQC 不识别；Per base sequence quality 列名与数据错位（`lower/upper` 实为 IQR fence 非 10th/90th 百分位）。

## 7. Low 发现

- **FASTQC-L01**：模板 GitHub 链接拼写错误 `hhttps://`（`report.html.tera:48`）。
- **FASTQC-L02**：内联外部 CSS/JS 无 SRI、无离线回退。
- **FASTQC-L03**：`FastqAnalysisResult::new` 12 参数 + `analyze`/`process` 逻辑几乎逐行重复（clippy `too_many_arguments`）。
- **FASTQC-L04**：`quartiles` 注释自认 lower/upper 实为 1.5×IQR fence 而非 min/max，UI 却当 min/max，与 FastQC 10th/90th 百分位语义不同（`process.rs:204-206` TODO）。
- **FASTQC-L05**：lib 构建下 `process`/`quartiles` dead_code 误报（`pub(crate)`）。
- **FASTQC-L06**：README/CLAUDE.md 未提 `--no-html`/`analyze` 库 API/`>>Seqkit Statistics`；作者邮箱 Cargo.toml 与 main.rs 不一致。
- **FASTQC-L07**：`mean_read_quality` 用 `u64` 截断除法丢精度（`process.rs:166`）。

## 8. 主仓实际调用契约

主仓 Snakemake 规则（`inst/rules/01fqcAtfirst.smk`、`03-0-fqcAtclean.smk`）只使用 `--no-html` + `-s` 模式。整改后命令使用 Snakemake 路径 quoting：

```text
fqc -q {input.R1:q} -s {params.R1_dir:q} --no-html
```

输出契约保持为 `<summary-dir>/fastqc_data.txt`。静态测试同时固定两条命令的 `:q` 参数和 raw/clean 四个 `fastqc_data.txt` 输出路径。

qctb 解析器定位 `>>Seqkit Statistics` 模块，并消费 `num_seqs`、`sum_len`、`Q20(%)`、`Q30(%)`、`min_len`、`avg_len`、`max_len`。本轮使用真实 run4 文件直接调用 qctb parser，结果为：

```text
num_seqs=200 sum_len=20200 min_len=101 avg_len=101 max_len=101 Q20=96 Q30=91
```

`>>END_MODULE` 可独立成行或紧贴数据行；qctb 均可解析。构建产物契约未变：`scripts/build-all-submodules.sh` 构建 `fqc`，`scripts/install.sh` 安装 `fqc`。

## 9. 外部兼容验证

- **SeqKit 2.13.0**：同一 `example.fastq` 的核心字段一致：`FASTQ/DNA/200/20200/101/101/101/Q20 96/Q30 91/AvgQual 19.94/GC 46.69/sum_n 228`；`N50_num=1` 与当前 SeqKit length-bin 实现一致。
- **Babraham FastQC 0.12.1**：Total Sequences 200、Sequence length 101、`%GC` 四舍五入为 47、position 1 mean quality 30.135 对照一致。`fqc` 是 FastQC-compatible subset，不声明 tile、duplication、adapter 等全部模块 parity。
- **MultiQC 1.35**：真实读取 `fastqc_data.txt`，发现 1 个报告并生成 `multiqc_fastqc.txt`、general stats、sources 和 HTML；未知 `Seqkit Statistics` 扩展不破坏 parser。
- **CI**：workflow 固定上述版本与 Java 21/Python 3.12，并在每次兼容 job 运行逐字段断言和大输入预算。固定 URL 的下载可用性仍需 GitHub-hosted runner 首次执行确认。

## 10. 完成门禁

- [x] Critical/High/Medium 风险为零。
- [x] 非法/IUPAC 碱基不再 panic。
- [x] malformed plain FASTQ 与 truncated gzip fail closed，失败不发布 summary。
- [x] Q20/Q30 为碱基级，AvgQual 对齐 SeqKit。
- [x] Per base sequence content 使用 A-T 和 G-C。
- [x] summary 输出排序确定，位置与百分比语义对齐 FastQC。
- [x] strict clippy、locked check/test/build 和 diff check 通过。
- [x] HTML 生成不请求网络，不因 CDN 失败 panic。
- [x] k-mer、per-position、read length 和 distinct length 有 typed 上限。
- [x] 本机大输入时间/RSS预算通过，CI 有同等机器 gate。
- [x] 真实 FastQC/MultiQC/SeqKit 差分门禁通过。
- [x] qctb 真实产物消费和主仓 Snakemake 静态契约通过。
- [ ] GitHub Actions compatibility job 远端实跑。
- [ ] commit/push 与主仓 submodule pointer 更新（需用户明确授权）。
