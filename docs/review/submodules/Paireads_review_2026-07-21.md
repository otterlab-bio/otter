# Paireads 系统审查报告（2026-07-21）

## 1. 审查结论

- **仓库基线**：`b4655fa47222fd682fd521504604870ead490ae0`
- **分支**：`master`
- **审查状态**：代码整改与本地动态验证已完成，整改提交 `1ece0ba` 已推送，主仓子模块指针纳入本次发布提交；Bismark extractor 集成仍受当前工具链缺失阻塞
- **发布结论**：Paireads 已解除已知 Critical/High/Medium 代码阻塞并完成子仓发布；主仓发布仍待真实 Bismark/Snakemake 集成验证
- **已确认问题**：已知 0 Critical、0 High、0 Medium

已完成的核心整改包括：输入/输出别名拒绝、目标目录 staging、批量发布与回滚、sort/index/close 错误传播、加固版 `bamdriver-go`、query-name 外部排序、single/dual 流式处理、CLI help/unknown-flag 行为和 CI 质量门禁。Dual BAM 的契约已明确为两个 BAM 中共有的唯一 primary mapped read names，不再声称验证 SAM `proper pair`。

## 2. 范围和数据流

### Single BAM 模式

```text
input BAM
  → 64 MiB 有界外部 query-name sort
  → 一次只读取一个 read-name group
  → 保留恰好一个 primary mapped R1 + 一个 primary mapped R2 的 group
  → 立即写入 staged BAM，并流式记录被过滤名称
  → name/coordinate sort + 可选 BAI
  → 批量发布 BAM、名称文件、BAI 或陈旧 BAI 删除操作
```

### Dual BAM 模式

```text
R1 BAM → 64 MiB 有界 query-name sort ┐
                                      ├→ 双路流式名称归并 → staged R1/R2 BAM
R2 BAM → 64 MiB 有界 query-name sort ┘                  → sort/index → 批量发布
```

Single mode 的业务契约是“完整唯一 primary mates”，Dual mode 的业务契约是“两个 BAM 中都存在的唯一 primary mapped read name”。两者都只输出 primary mapped records，不宣称 SAM `FlagProperPair` 校验。

重点审查路径：

- `cmd/paireads/main.go`
- `cmd/paireads/main_test.go`
- `bamnative/` 与 `internal/bgzip/` 对 `bamdriver-go` 的兼容封装
- `go.mod`
- `.github/workflows/ci.yml`
- 主仓 `inst/rules/build_methy_matrix_bismark.smk`
- 主仓 `inst/rules/pdx_build_methy_matrix_bismark.smk`

## 3. 已执行证据

### 3.1 Go 门禁

以下命令已通过：

```bash
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
"$(go env GOPATH)/bin/govulncheck" ./...
```

本地 samtools 兼容门禁也已通过：`quickcheck`、record count、`SO:coordinate` 和 BAI 区域查询。主仓 race、test、vet 和漏洞扫描同样通过；xenofilter-go 的 test、vet 和 race 通过。

### 3.2 依赖升级验证

当前 `go.mod` 锁定已审查的加固提交：

```text
github.com/otterlab-bio/bamdriver-go v0.1.2-0.20260721055359-a22f77784fc4
```

模块路径同步迁移为：

```text
module github.com/otterlab-bio/Paireads
```

### 3.3 动态与回归证据

新增测试覆盖：

- 同字符串路径、symlink、hardlink 文件 identity 拒绝，且源文件内容不变。
- 非坐标排序仍将待删除的 BAM index 目标纳入身份校验，覆盖 single direct/symlink/hardlink 与 dual 输入冲突。
- 批量发布第二阶段失败时恢复旧输出；备份与恢复同时失败时联合传播两类错误。
- single mode 完整 primary mates、被过滤名称和合法空 BAM 输出。
- dual mode matched-name 流式归并。
- name-sorted 输出事务性删除陈旧 BAI。
- `--help` 正常返回，未知 flag 明确报错。
- 最终 name/coordinate sort 显式使用 64 MiB 内存预算，并将 sort 临时目录限制在 staged 输出目录。

## 4. Critical 发现

### PAIR-C01：允许输入和输出为同一路径，会截断并破坏源 BAM — **已修复**

- **修复状态**：在任何读写前执行 canonical path、symlink 和 `os.SameFile` identity 校验；所有 BAM/BAI/名称文件先写入目标目录 staged 路径，再批量发布。回归测试覆盖相同路径、symlink、hardlink 和源内容不变。

- **位置**：`cmd/paireads/main.go:97-132`、`cmd/paireads/main.go:474-559`
- **根因**：第一遍分析关闭后，第二遍 `filterBAMWithCount` 先打开 input，再通过 `NewWriter(outputPath, header)` 创建 output。当路径相同时，writer 创建动作会截断 reader 正在读取的同一文件。
- **动态复现**：
  - 输入为合法、包含一对 primary records 的 278 字节 BAM。
  - 执行 `paireads input.bam input.bam`。
  - 程序报告 `Complete primary pairs: 1`，随后 `Kept 0 out of 2 records`。
  - 原文件变为 199 字节的空 BAM。
  - name sort 报 warning，进程最终 `exit_status=0`。
- **影响**：用户原始 BAM 可被不可恢复地替换为空 BAM；工作流可能继续消费被破坏的文件。
- **整改**：
  1. 在任何写操作前，通过绝对路径、`filepath.EvalSymlinks` 和文件 identity 拒绝 input/output 指向同一文件。
  2. 永远写入目标目录中的临时文件，完成 close/sort/index/quickcheck 后原子替换最终路径。
  3. 同时拒绝 dual mode 中 R1/R2/output 之间的任意路径别名。
- **回归测试**：同字符串路径、相对/绝对别名、符号链接、硬链接四类测试；断言命令非零退出且源文件哈希不变。

## 5. High 发现

### PAIR-H01：sort/index 失败只打印 warning，命令仍成功退出 — **已修复**

- **修复状态**：`run`、single 和 dual mode 全部返回 error；sort、index、write、flush、close 和 publish 失败逐层传播。加固 sorter 将空 BAM 作为合法输入处理，并有端到端测试。

- **位置**：`cmd/paireads/main.go:135-149`、`cmd/paireads/main.go:225-248`
- **证据**：single/dual 两种模式均吞掉 `nameSort`、`sortAndIndex` 错误并继续打印 `Done!`。
- **动态复现**：只有一个不完整 mate 的输入产生空 BAM；旧 Sort 返回 `no records to sort`，程序打印 warning 并 `exit_status=0`。
- **影响**：磁盘满、rename 失败、索引失败或排序失败均可被工作流误判为成功。主仓随后直接运行 `bismark_methylation_extractor`，错误会延迟且难定位。
- **整改**：主执行函数返回 error；任何必需 sort/index 失败都必须非零退出。空 BAM 应被 sorter 正确视为合法输入，而不是靠忽略错误继续。
- **回归测试**：空输出、不可写目录、注入 rename/index 失败，断言非零退出或明确支持的空 BAM 成功路径。

### PAIR-H02：仍锁定加固前的 `bamdriver-go v0.1.0` — **已修复**

- **修复状态**：升级到 `v0.1.2-0.20260721055359-a22f77784fc4`，获得严格 parser、空 BAM 支持、有界外部排序、加固索引与相关 fuzz 修复。

- **位置**：`go.mod:5`
- **代码证据**：旧版本 reader 对 header text、reference name、record block size 直接按输入长度分配；BGZF 对结构和解压大小缺少当前版本的上限；排序将所有 records 加载到内存；`IsSorted` 只信 header。
- **影响**：恶意或损坏 BAM 可触发无界分配/崩溃；大 BAM 排序 OOM；错误 sort header 可导致错误索引和输出；缺失当前版本的原子 BAI、严格 CIGAR/aux/header 验证及 fuzz 修复。
- **整改**：升级到已审查的 `bamdriver-go@a22f777` 或后续正式 release，运行 tidy 并将 samtools/fuzz 门禁固定在 CI。
- **回归测试**：复用 bamdriver 的 malformed BAM/BGZF、排序、BAI 和 samtools fixtures；下游全量测试必须通过。

### PAIR-H03：Dual BAM 模式仅计算 read-name 交集，却称其为 properly paired — **已修复**

- **修复状态**：业务契约冻结为 matched-name filtering；帮助、日志、摘要和 golden test 均只描述“两个 BAM 中共有的唯一 primary mapped 名称”，不再使用 properly paired 术语。

- **位置**：`cmd/paireads/main.go:170-267`、`cmd/paireads/main.go:277-315`、`cmd/paireads/main.go:394-424`
- **证据**：`extractReadNames` 只要求每个 BAM 中存在唯一 primary mapped record；没有验证 `FlagPaired`、R1/R2 mate flag、`FlagProperPair`、mate reference/position、template relationship 或两个 header/reference dictionary。
- **影响**：两个无关 alignment 只要 read name 相同就会被标记为 properly paired 并保留；反之合法复杂 alignment 可能因重复 primary 被整个拒绝。结果语义与 README 和日志不符。
- **整改**：先冻结 dual mode 的真实业务契约：
  - 若只需要名称交集，重命名为 matched names，不得称 properly paired。
  - 若需要配对正确性，则验证 R1/R2 flags、mate identity、header compatibility，并为异常名称明确分类。
- **回归测试**：同名但均为 R1、未设置 paired flag、不同 reference dictionary、mate coordinate 不一致、proper-pair flag 缺失等 fixtures。

### PAIR-H04：read-name 集合和排序均为无界内存实现 — **已修复**

- **修复状态**：single mode 使用外部 query-name sort 后逐 group 处理；dual mode 对两个已排序流做双路归并。排序内存上限固定为每次 64 MiB，常驻名称数据仅为当前 group。

- **位置**：`cmd/paireads/main.go:277-424`；依赖 `bamdriver-go v0.1.0/pkg/bamnative/sort.go:31-66`
- **证据**：single mode 为全部名称保存 status map；dual mode 为两份 BAM 分别保存名称 set 和交/差集；旧 sorter 将全部 records 存入 slice 后排序。
- **影响**：大规模 BAM 会使用与 fragment 数量线性增长的内存；排序再叠加完整 record 内存，可能 OOM。README 的“minimal memory”和“external sorting”声明与当前依赖实现不符。
- **整改**：升级 bamdriver 获得有界外部排序；对 name filtering 使用 query-name sorted 流式处理，或建立有界磁盘索引/分区。
- **回归测试**：百万级唯一名称 fixture，设置明确内存预算并记录峰值；验证输出确定性。

### PAIR-H05：输出和替换流程非原子，writer close 错误被忽略 — **已修复**

- **修复状态**：所有 writer 显式 close 并检查错误；single/dual 全部产物采用同目录 staging、旧产物 backup、批量 rename 和失败回滚。name-sorted 发布还事务性移除陈旧 BAI。

- **位置**：`cmd/paireads/main.go:474-559`、`cmd/paireads/main.go:562-603`
- **证据**：filter 直接写最终 output，使用 `defer writer.Close()` 且不检查 close error；sort/nameSort 先 `os.Remove(bamPath)` 再 `os.Rename(sortedPath, bamPath)`；dual mode 的 R1 成功后 R2 失败会留下半套产物。
- **影响**：flush、磁盘、rename 或 index 失败时，最终路径可能是部分 BAM、缺少 BAI，或者原输出已被删除。
- **整改**：所有阶段在目标目录私有临时路径完成；显式检查 close/sync；验证后使用 rename 安装；dual outputs 采用批次提交或明确清理已完成的另一侧。
- **回归测试**：writer close、磁盘写入、rename、BAI 和第二输出失败注入，断言旧产物保持不变且不会出现半成功。

## 6. Medium 发现

### PAIR-M01：Single mode 的“proper pair”实际只表示唯一 primary R1+R2 — **已修复**

- **修复状态**：CLI 和 README 明确使用“complete unique primary R1/R2 mates”，并明确不检查 `FlagProperPair`。

- **位置**：`cmd/paireads/main.go:317-356`
- **证据**：`classifyCompletePairs` 不检查 `FlagProperPair`、mate-unmapped、mate reference/position 或 template consistency。
- **影响**：实现可能满足 Bismark 所需的“完整双端名称”，但文档和 CLI 使用了更强的 SAM proper-pair 术语，容易误用。
- **整改**：冻结主仓需求后调整命名/文档，或实现真正 proper-pair 校验。

### PAIR-M02：过滤会静默删除 retained name 的 secondary、supplementary 和 unmapped records — **已修复（契约明确）**

- **修复状态**：CLI、README 和审查报告明确输出为 primary mapped-only 投影；single/dual golden tests 固定该行为。

- **位置**：`cmd/paireads/main.go:505`、`cmd/paireads/main.go:545`
- **影响**：输出不是简单的 read-name 过滤结果，而是 primary mapped-only 投影；当前 README 未明确该数据损失。
- **整改**：明确业务契约。若名称被保留时应保留所有 associated records，则分离“决定名称使用 primary”与“写出全部记录”两个步骤。

### PAIR-M03：模块路径仍为历史名称 `github.com/PeeperLab/xenofilter` — **已修复**

- **修复状态**：`go.mod`、源码 imports、测试与 CI generator 已迁移到 `github.com/otterlab-bio/Paireads`。

- **位置**：`go.mod:1` 及源码 imports
- **影响**：发布、依赖识别、漏洞报告和维护者理解均混淆，仓库名称与 module identity 不一致。
- **整改**：迁移到 `github.com/otterlab-bio/Paireads`（或最终规范路径），同步 imports 和 CI generator。

### PAIR-M04：手写参数解析使标准 CLI 行为异常 — **已修复**

- **修复状态**：`--help`/`-h` 和 `--version`/`-V` 成功返回；未知 flag 与错误参数数量返回明确错误，并有回归测试。

- **位置**：`cmd/paireads/main.go:16-52`
- **动态证据**：`--help` 打印帮助但状态码为 1；未知 flag 可能被当作 positional argument。
- **整改**：使用标准 `flag`、Cobra 或等价 parser，定义 `--help`、`--version`、未知 flag 和参数数量契约。

## 7. 测试覆盖缺口

已覆盖的关键回归包括：路径别名、single/dual 端到端、matched-name 语义、合法空输出、publication rollback、陈旧 BAI 删除、CLI 行为和 BAM writer round-trip。

仍建议追加但不再属于已知 Critical/High 阻塞：

- 独立进程百万级 fixture 的峰值 RSS 记录。
- header/reference dictionary 兼容策略（dual mode 当前按名称契约独立保留各自 header）。
- writer close、索引和磁盘故障的更深注入测试。
- 主仓普通与 PDX Bismark extractor 最小集成。
- 复用上游 `bamdriver-go` malformed BAM/BGZF fuzz corpus 的下游周期性验证。

## 8. 剩余发布步骤

1. 在具备 `bismark_methylation_extractor` 与 Snakemake 的环境运行主仓普通与 PDX Bismark extractor 最小集成；当前环境仅具备 samtools 1.24，未安装 `bismark_methylation_extractor`、`snakemake`、`enva` 或 `conda`。
2. `Paireads` 整改已以 `1ece0ba` 提交并推送；主仓 submodule 指针纳入本次发布提交。
3. 按计划进入其余子仓系统审查。

## 9. 完成门禁

- [x] PAIR-C01 有回归测试且源文件内容保持不变。
- [x] 0 Critical / 0 High / 0 Medium。
- [x] 升级到已审查 `bamdriver-go` 版本。
- [x] sort/index/write/flush/close/publish 错误返回非零，批量发布失败恢复旧产物。
- [x] single/dual 语义有明确 CLI、README 和 golden tests。
- [x] `go test -count=1 ./...`、`go vet ./...` 通过。
- [x] `go test -race -count=1 ./...` 通过。
- [x] `govulncheck ./...` 通过，无可达漏洞。
- [x] 本地 samtools quickcheck、view count、sort check 和 BAI 区域查询通过。
- [ ] 主仓普通与 PDX Bismark methylation extractor 最小集成通过。
