# 子仓库系统审查计划（2026-07-21）

## 1. 目标

对 `xdxtools` 的全部 Git 子仓库建立统一、可复核、可追溯的审查流程，重点确认：

1. 生物信息学计算结果正确，边界条件和格式约定明确。
2. 非法、截断或超大输入不会导致静默错误、崩溃或无界资源消耗。
3. 输出格式可被约定的外部工具读取，并与参考实现保持一致。
4. 并发、临时文件、缓存、索引和失败恢复行为安全可靠。
5. 子仓库修复经过独立验证，并在主仓工作流中完成集成回归。

“能够编译”或“曾修复过一个 CI 问题”不等同于完成系统审查。只有满足本计划的完成定义后，子仓库状态才能标记为“已审查”。

## 2. 范围与当前状态

`.gitmodules` 当前登记 9 个子仓库。

| 子仓库 | 主要职责 | 当前状态 | 审查优先级 |
|---|---|---|---|
| `bamdriver-go` | BAM/BGZF、BAI/FAI、排序与 NM 计算 | 已完成系统审查和整改，作为本计划基线 | 已完成 |
| `xenofilter-go` | PDX host/graft reads 分类和过滤 | 整改提交 `6ae7f84` 已推送，主仓指针纳入本次提交；0 Critical / 0 High / 0 Medium，待真实 PDX 集成 | P0 |
| `Paireads` | 两个 BAM 的 read-name 配对、过滤、排序和索引 | 整改提交 `1ece0ba` 已推送，主仓指针纳入本次提交；0 Critical / 0 High / 0 Medium，待 Bismark 集成 | P0 |
| `methrix-cli` | Bismark 甲基化数据转换、自定义 HDF5 与 QC 输出 | 首轮整改完成，原生 schema/规模化/集成待完成 | P0 |
| `htseq2matrix-go` | HTSeq 计数合并、基因 ID 转换和矩阵输出 | I/O 可靠性整改完成，科学矩阵契约待冻结 | P1 |
| `fastqc-rs` | FASTQ 质量统计、HTML 与 MultiQC 输出 | 本地系统整改 closure 完成；异常输入、资源预算、外部消费者与主仓静态契约通过 | P1 |
| `qctb` | RRBS/WGBS/RNA-seq QC 汇总 | 当前主仓契约整改完成，原生报告 schema 待冻结 | P1 |
| `gomats` | rMATS 任务构造和可变剪接流程编排 | 输入/路径整改完成，任务事务和产物契约待完成 | P1 |
| `enva` | 环境创建、发现、执行、安装和删除 | 路径/argv 首轮整改完成，完整测试与事务安全待完成 | P0 |

六个 Wave 2-3 子仓均已完成只读审查和首轮整改；当前状态、剩余问题与实施批次见 `wave2_wave3_remediation_plan_2026-07-22.md`。`xenofilter-go` 与 `Paireads` 已完成代码整改、本地门禁和子仓推送，尚待具备工具链环境中的真实工作流集成。

## 3. 审查原则

### 3.1 证据优先

每个结论必须对应以下一种或多种证据：

- 可定位的代码路径和触发条件。
- 可重复的单元测试或回归测试。
- 与参考实现或外部标准工具的差分结果。
- 竞态检查、模糊测试、静态分析或依赖安全扫描结果。
- 主仓真实调用路径的集成测试结果。

不以 README 声明、人工目测或一次成功运行替代测试证据。

### 3.2 先审查，后整改

每个子仓采用两阶段提交：

1. 审查阶段：记录问题、严重级别、复现方式、影响范围和建议方案。
2. 整改阶段：添加失败测试，实施修复，再运行完整门禁。

发现严重数据正确性问题时可立即修复，但报告中仍需保留原始问题、失败测试和修复验证。

### 3.3 依赖顺序

涉及多个仓库时按以下顺序提交和验证：

1. 最底层库或子仓库。
2. 直接消费者。
3. 主仓子模块指针和工作流集成。

主仓不得先指向尚未推送或尚未通过下游测试的子仓库提交。

### 3.4 隔离和非破坏性

- 文件删除、环境删除和缓存清理必须在临时目录或隔离环境中测试。
- 不使用生产数据、用户真实环境或共享集群目录验证破坏性路径。
- 外部命令测试必须使用显式参数数组，避免通过 shell 拼接不可信路径。
- 测试产生的临时文件必须自动清理；失败时保留必要诊断信息。

## 4. 统一审查门禁

每个子仓库必须覆盖以下五类门禁。

### Gate A：代码与架构

- 明确 CLI 入口、核心数据流、状态边界和外部依赖。
- 检查错误是否被传播，是否存在静默降级、吞错或部分成功。
- 检查整数转换、索引、切片、空值、超大输入和资源上限。
- 检查重复实现，尤其是 BAM/BGZF/FAI/BAI 等已有公共实现。
- 检查临时文件、原子替换、关闭顺序和失败清理。
- 检查公共 API、CLI 参数和输出契约是否与主仓调用一致。

### Gate B：测试与动态验证

Go 子仓最低要求：

```bash
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
```

Rust 子仓最低要求：

```bash
cargo fmt --check
cargo clippy --all-targets --all-features -- -D warnings
cargo test --all-features
```

按输入解析面增加定向 fuzz/property 测试。若功能依赖系统库或外部程序，应在 CI 中显式安装，并分别记录“已执行”和“因环境缺失跳过”的测试；发布门禁不得静默跳过关键兼容性测试。

### Gate C：科学结果和格式兼容性

- 为核心算法建立小型、人工可核算的 golden fixtures。
- 与原始 R/Python/Java 实现或标准命令行工具做差分测试。
- 明确坐标系、链方向、缺失值、重复记录、排序和舍入规则。
- 输出按格式规范检查，并由真实消费者读取验证。
- 多线程和单线程对同一输入必须产生等价结果；要求确定性时应逐字节一致。

### Gate D：安全性与可靠性

- 检查路径穿越、符号链接、命令注入、危险递归删除和权限边界。
- 检查压缩炸弹、超大记录、无界集合、过度线程和磁盘耗尽风险。
- 检查下载来源、TLS、校验和、缓存污染及供应链依赖。
- 检查并发访问、锁、竞态、死锁和重复执行的幂等性。
- 运行语言对应的依赖漏洞检查；工具不可用时需在报告中记录缺口。

### Gate E：主仓集成与发布

- 使用主仓实际参数调用子工具，而不只测试子仓 README 示例。
- 覆盖 `scripts/build-all-submodules.sh` 和 `scripts/install.sh` 的产物名称。
- 验证 `--help`、版本输出、退出码、标准输出和标准错误契约。
- 至少完成一个最小真实工作流或可复核 dry-run。
- 子仓提交推送后，更新主仓子模块指针并重新运行主仓相关测试。

## 5. 问题严重级别

| 级别 | 定义 | 处理要求 |
|---|---|---|
| Critical | 可造成不可恢复的数据破坏、任意命令执行、危险目录删除或大范围错误结果 | 立即停止发布；修复后执行完整回归 |
| High | 核心科学结果错误、格式不兼容、常见输入崩溃、并发数据损坏 | 当前审查波次内修复，不允许带入发布 |
| Medium | 边界输入错误、错误信息不足、资源控制缺失、非关键兼容性问题 | 原则上当前整改；延期必须记录负责人和门禁 |
| Low | 可维护性、文档、非关键性能和开发体验问题 | 可进入后续 backlog，但需明确记录 |

Critical 和 High 问题未清零时，不得将对应子仓库标记为“已审查”。

## 6. 分波次执行计划

### Wave 0：基线和工具准备

目标：所有子仓使用同一套报告结构、严重级别和完成定义。

任务：

1. 为每个子仓记录当前提交、分支、语言版本和依赖锁文件状态。
2. 建立最小测试命令和 CI 环境依赖清单。
3. 确认参考实现、标准工具及可公开提交的小型 fixtures。
4. 确认主仓中调用该子工具的规则、参数和预期产物。
5. 为后续报告准备统一模板和证据目录命名规则。

完成标准：8 个待审子仓均有明确的审查入口、参考 oracle 和可执行测试基线。

### Wave 1：BAM 数据链

执行顺序：`xenofilter-go` → `Paireads`。

原因：二者处理 BAM 并直接影响 PDX reads 保留结果；`Paireads` 还包含自有 BAM/BGZF 实现，存在与已整改 `bamdriver-go` 分叉的风险。

#### 6.1 `xenofilter-go`

重点审查：

- graft/host BAM 的配对规则、read-name 同步和单端/双端分类状态机。
- 缺失 mate、secondary/supplementary alignment、重复 read name、未比对 reads 的语义。
- NM tag、插入、软剪切、bisulfite 转换和阈值边界。
- 使用参考 FASTA 重算 NM 时的坐标、链方向、越界和索引行为。
- 多线程结果是否与单线程一致，统计计数是否存在竞态。
- 输出 BAM header、排序、索引、临时文件和失败后的部分产物。
- 与原始 XenofilteR 参考结果的差分测试。

最低兼容性证据：

- 人工可核算的单端和双端分类 fixtures。
- 普通测序和 bisulfite 模式 fixtures。
- `samtools quickcheck`、`samtools view` 和索引区域查询。
- `go test -race` 和分类/NM 输入 fuzz 测试。

完成标准：分类结果、统计表和输出 BAM 均通过参考结果与 samtools 验证。

#### 6.2 `Paireads`

重点审查：

- “同名 reads”与“properly paired”概念是否混用，行为是否符合主仓需求。
- 多 alignment、secondary/supplementary、重复 read name 和 read group 的处理。
- 两个输入 BAM header/reference dictionary 不一致时的行为。
- read-name 集合的内存上限，以及 README 所述流式/外部排序是否与实现一致。
- 输出排序稳定性、BAI 正确性、空输入和单侧缺失 reads。
- 自有 `bamnative`/BGZF 代码与 `bamdriver-go` 的差异和重复维护风险。
- 写入失败、磁盘不足和中断时的原子性与清理。

整改决策点：

- 优先评估直接依赖 `bamdriver-go`，避免继续维护第二套 BAM/BGZF/BAI 实现。
- 若保留自有实现，必须达到 `bamdriver-go` 已建立的格式、fuzz 和 samtools 门禁。

最低兼容性证据：

- 重复名称、多 alignment、空交集和全交集 fixtures。
- 输出 BAM 的 `samtools quickcheck`、排序检查和区域查询。
- 大量唯一 read names 的有界内存测试。

完成标准：配对语义有文档和 golden tests，输出 BAM/BAI 可由 samtools 稳定读取。

### Wave 2：科学计算与报告输出

可在 Wave 1 完成后并行审查：`methrix-cli`、`htseq2matrix-go`、`fastqc-rs`、`qctb`。

#### 6.3 `methrix-cli`

重点审查：

- Bismark coverage 解析、压缩输入、染色体命名、排序和重复 CpG。
- 0-based/1-based 坐标转换、CpG strand 合并和参考基因组提取。
- coverage、methylated count、beta 值的数值范围、溢出、精度和缺失值。
- `u16` coverage 是否会截断真实高覆盖数据。
- 并行样本处理的确定性、内存峰值和失败传播。
- 自定义 HDF5 schema 的 dataset 名称、维度、类型、字符串编码、chunk/compression 和 metadata。
- Rust 原生 validator/reader 与 `qctb` 对正式 schema 的消费兼容性。
- QC Excel 和 CpG annotation 的字段、公式、空样本和零分母。
- 下载参考基因组的 TLS、校验和、断点/部分文件和原子安装。

最低兼容性证据：

- 小型 Bismark fixture 的人工可核算数值验证。
- Rust 原生 validator 读取生成 HDF5，并校验维度、坐标、样本名和数值。
- 单线程/多线程结果等价。
- 极端 coverage、空文件、截断 gzip 和重复 CpG 回归测试。

完成标准：版本化 custom HDF5 schema 与 QC 产物通过原生 validator 和 `qctb` 消费验证，关键数值与人工 golden fixtures 一致。

#### 6.4 `htseq2matrix-go`

重点审查：

- HTSeq 两列输入、特殊统计行、空白、重复 gene ID、负数和超大计数。
- 样本名从文件名推导时的冲突和确定性排序。
- 合并语义究竟是 union、left join 还是 intersection，缺失计数如何填充。
- ENSEMBL 版本后缀、未映射 ID、一对多/多对一映射和重复 symbol 聚合规则。
- “取最大值”是否与原 R 实现一致，是否应求和或保留独立记录。
- `log2(x+1)` 对零、负数、溢出、NaN/Inf 的行为。
- 内嵌人/鼠数据库的来源、版本、生成过程和可复现性。
- TSV quoting、列顺序、换行和跨平台确定性。

最低兼容性证据：

- 与原 R 版本对同一 fixture 的矩阵逐单元格比较。
- 重复 ID、缺失基因、样本名冲突和特殊 HTSeq 行 fixtures。
- 嵌入数据库版本和生成哈希可追溯。

完成标准：原始矩阵和标准化矩阵与冻结的参考输出一致，列顺序稳定。

#### 6.5 `fastqc-rs`

重点审查：

- FASTQ 四行结构、多行/异常记录、CRLF、空 reads、截断 gzip 和非法碱基。
- Phred 编码识别、质量范围、不同 read 长度和超长 reads。
- 每碱基质量、GC、N、sequence content 和 k-mer 统计公式。
- 大文件的流式行为、内存上限、计数溢出和 gzip 解压限制。
- HTML 输出转义、离线资源、自包含声明和恶意 read name/content。
- `fastqc_data.txt` 字段与 MultiQC parser 的真实兼容性。
- 与 Babraham FastQC 的差异是设计差异还是实现错误。

最低兼容性证据：

- 同一组 FASTQ 同时运行 `fqc` 和标准 FastQC，比较核心统计。
- 使用 MultiQC 真实解析生成的 summary。
- gzip/非 gzip、可变 read 长度、低质量和 malformed fixtures。

完成标准：核心指标偏差有明确解释，MultiQC 可稳定解析，异常 FASTQ 明确失败而非产生误导报告。

#### 6.6 `qctb`

重点审查：

- Bismark、STAR 及主仓实际日志格式的解析完整性。
- 缺失字段、版本差异、重复字段、千分位和百分号处理。
- 零分母、NaN/Inf、舍入、百分比和汇总公式。
- 样本与 pdata/config 的匹配、重复样本和缺失样本。
- RRBS/WGBS/RNA-seq 模式分支是否选择正确指标。
- Excel sheet 名称、单元格类型、TSV 列顺序和跨运行确定性。
- 与版本化原生 `qctb.report` schema golden fixtures 的逐字段验证。

最低兼容性证据：

- 各工作流至少一组真实精简日志 fixture。
- 对版本化原生输出逐字段比较并记录允许的舍入误差。
- 缺字段、空样本和不同 STAR/Bismark 版本回归测试。

完成标准：主仓所有支持模式均有 golden output，Excel/TSV 契约稳定。

### Wave 3：运行环境与流程编排

执行顺序可并行：`gomats` 与 `enva`。

#### 6.7 `gomats`

重点审查：

- pdata/Excel 解析、分组、重复样本、空组和 pairwise combination 生成。
- BAM 路径匹配、缺失文件、样本顺序和物种字段。
- sequence-length/N50 计算边界与参考数据来源。
- rMATS 参数构造、路径空格、特殊字符和 shell 注入。
- 本地/Slurm 执行、并发上限、退出码、日志和部分任务失败。
- dry-run 是否完整反映真实命令，重跑是否幂等。
- 临时 b1/b2 文件、输出目录冲突和失败清理。
- rMATS 版本差异及产物完整性检查。

最低兼容性证据：

- 使用可控 fake `rmats.py` 捕获参数数组并验证所有组合。
- 单组、双组、多组、重复样本和缺失 BAM fixtures。
- 至少一个小型真实 rMATS dry-run 或集成运行。

完成标准：任务矩阵和命令参数可由测试精确复核，错误任务不会被报告为成功。

#### 6.8 `enva`

重点审查：

- 环境名称、prefix 规范化、符号链接和允许操作根目录。
- `remove`、`create --force`、cache cleanup 是否可能越界删除。
- `run` 参数边界、shell hook/activate 输出转义和命令注入。
- 同名环境发现、优先级、adopt 和 ownership metadata 一致性。
- 并发 create/install/remove 的锁、事务性和中断恢复。
- YAML/package spec/channel 解析和依赖求解错误传播。
- 下载、缓存、TLS、校验和、部分文件与缓存污染。
- conda/mamba/micromamba/rattler 后端行为差异。
- Windows、Linux、macOS 路径与 shell 差异。

最低安全证据：

- 所有删除类测试在临时根目录执行，覆盖 `/`、空路径、`..` 和符号链接逃逸。
- 包含空格、引号、换行和 shell 元字符的命令/路径测试。
- 并发操作和中断恢复测试。
- 至少一个 rattler 原生路径和一个兼容后端的隔离 e2e。

完成标准：破坏性操作有明确安全边界，命令执行不经过不必要的 shell 拼接，状态更新具备事务性。

### Wave 4：跨仓集成和发布门禁

任务：

1. 更新所有已整改子仓提交，并按依赖顺序推送。
2. 更新主仓子模块指针。
3. 运行 `scripts/verify_toolchain_consistency.sh`。
4. 运行 `scripts/build-all-submodules.sh`，确认 8 个面向用户的子工具产物名称不变。
5. 运行主仓 Go 测试、静态检查和相关工作流测试。
6. 执行 RRBS local dry-run 与 RNASEQ slurm dry-run，并更新 release evidence。
7. 增加 PDX/BAM 链和甲基化 HDF5 链的最小集成证据。
8. 汇总仍接受的 Low/Medium 风险和发布例外。

完成标准：所有子仓报告状态与主仓子模块指针一致，发布证据可从干净环境重复生成。

## 7. 每个子仓的交付物

每次审查至少产生以下内容：

1. 审查报告：`docs/review/submodules/<repository>_review_<YYYY-MM-DD>.md`。
2. 发现清单：包含严重级别、代码位置、触发输入、影响和整改状态。
3. 回归测试：优先提交到对应子仓测试目录。
4. 验证记录：命令、工具版本、结果和跳过原因。
5. 兼容性证据：参考实现或标准工具的差分结果。
6. 集成影响：主仓调用、CLI 契约、工作流和文档是否需要更新。
7. 提交记录：子仓提交、推送状态和主仓子模块指针提交。

报告建议结构：

```text
# <repository> 审查报告
## 范围与基线提交
## 架构和数据流
## 发现摘要
## Critical/High/Medium/Low 发现
## 已实施整改
## 测试与兼容性证据
## 主仓集成结果
## 遗留风险与发布结论
```

大体积原始日志不直接粘贴进报告；报告记录命令、摘要、关键失败片段和可定位的证据路径。

## 8. 完成定义（Definition of Done）

单个子仓只有同时满足以下条件才能标记为“已审查”：

- [ ] 代码和数据流已覆盖，核心边界有明确结论。
- [ ] Critical 和 High 问题为零。
- [ ] Medium 问题已修复，或有书面延期理由、负责人和门禁。
- [ ] 新发现的问题有回归测试，不能只修改实现。
- [ ] 语言对应的测试、静态检查和竞态/并发检查通过。
- [ ] 核心解析面至少有 fuzz/property 或等价鲁棒性测试。
- [ ] 与参考实现或标准工具的兼容性证据通过。
- [ ] 主仓真实调用路径通过。
- [ ] 子仓提交已推送，主仓子模块指针已更新。
- [ ] 审查报告、证据和实际提交三者一致。

整个审查计划完成还要求：

- [ ] 9 个子仓均有明确状态，其中 `bamdriver-go` 的既有报告/整改作为基线引用。
- [ ] 8 个待审子仓全部达到单仓完成定义。
- [ ] 主仓构建、安装、工具链一致性和发布 dry-run 门禁通过。
- [ ] PDX/BAM、RNA-seq 计数/QC、甲基化 HDF5 三条关键数据链均有集成证据。

## 9. 进度记录

| 波次 | 子仓库 | 状态 | 报告 | Critical/High | 主仓集成 |
|---|---|---|---|---|---|
| 基线 | `bamdriver-go` | 已整改，待纳入统一索引 | 既有审查记录 | 0 | 已通过 |
| Wave 1 | `xenofilter-go` | 整改提交 `6ae7f84` 已推送，主仓指针已更新；待真实集成 | `submodules/xenofilter-go_review_2026-07-21.md` | 0 Critical / 0 High | 规则失败标记契约通过，真实 PDX 集成待工具链 |
| Wave 1 | `Paireads` | 整改提交 `1ece0ba` 已推送，主仓指针已更新；待真实集成 | `submodules/Paireads_review_2026-07-21.md` | 0 Critical / 0 High | samtools 1.24 兼容通过，Bismark extractor 集成待工具链 |
| Wave 2 | `methrix-cli` | 首轮整改完成，Rust 门禁通过；仍阻断发布 | `submodules/methrix-cli_review_2026-07-22.md` | 原子发布与 custom schema 已整改；原生 validator、分块写、genome hash 待完成 | 不要求 R loader；主仓/qctb 原生集成待验证 |
| Wave 2 | `htseq2matrix-go` | 首轮 I/O 整改完成，Go 门禁通过；仍阻断发布 | `submodules/htseq2matrix-go_review_2026-07-22.md` | count 与事务发布已整改；matrix/mapping 科学契约待冻结 | 主仓参数一致；human/mouse 原生 golden 待完成 |
| Wave 2 | `fastqc-rs` | 本地整改 closure 完成；GitHub-hosted compatibility job 与交付待完成 | `submodules/fastqc-rs_review_2026-07-22.md` | 0 Critical / 0 High / 0 Medium；malformed/truncated、资源上限、HTML 网络边界已关闭 | qctb 真实产物消费、FastQC 0.12.1/SeqKit 2.13.0/MultiQC 1.35 和主仓规则静态契约通过；commit/push/指针待授权 |
| Wave 2 | `qctb` | 首轮整改完成，Rust 门禁通过；原生 schema 待冻结 | `submodules/qctb_review_2026-07-22.md` | 当前 config/Methrix/FQC/原子输出已整改；其余 parser 和 golden 待完成 | 不要求历史 R/RDS；四模式主仓集成待验证 |
| Wave 3 | `gomats` | 首轮输入与路径整改完成，Go 门禁通过；仍阻断发布 | `submodules/gomats_review_2026-07-22.md` | shell 边界、pdata/BAM、路径、零任务已整改；staging/产物契约待完成 | 真实 rMATS 与 PDX 集成待工具链 |
| Wave 3 | `enva` | 首轮路径和 argv 整改完成；完整测试待稳定 | `submodules/enva_review_2026-07-22.md` | Critical 路径逃逸与 argv 注入已整改；transaction/lock/cache/download 待完成 | fmt/clippy 通过；test 79/81 后补丁待重跑 |
| Wave 4 | 主仓跨仓集成 | 待开始 | 汇总报告待创建 | 不适用 | 待验证 |

每完成一次审查或整改，应立即更新本表和 `docs/active_context.md`，不得仅依赖聊天记录或未提交的本地日志维护进度。
