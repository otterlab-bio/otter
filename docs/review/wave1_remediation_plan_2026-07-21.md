# Wave 1 整改执行计划（2026-07-21）

## 1. 范围和目标

本计划处理 Wave 1 两个 BAM 数据链子仓库的已确认问题：

- `xenofilter-go`：0 Critical、7 High、4 Medium。
- `Paireads`：1 Critical、5 High、4 Medium。

审查报告：

- [`xenofilter-go_review_2026-07-21.md`](submodules/xenofilter-go_review_2026-07-21.md)
- [`Paireads_review_2026-07-21.md`](submodules/Paireads_review_2026-07-21.md)

目标不是只让测试恢复绿色，而是清零 Critical/High，并建立能够阻止同类回归的自动门禁。

## 2. 执行原则

1. **测试先行**：每个已确认问题先加入失败回归测试，再修改实现。
2. **先止损再重构**：先处理数据破坏和错误成功状态，再处理内存架构。
3. **底层依赖优先**：`bamdriver-go` 已整改；先让 `Paireads` 升级依赖，再验证上层流程。
4. **原子产物**：BAM、BAI 和批次输出只在全部验证完成后安装到最终路径。
5. **主仓契约冻结**：`Paireads` 配对语义和 XenofilteR threshold 必须通过主仓实际需求与参考实现确认，不能只靠变量名猜测。
6. **提交顺序**：子仓提交并推送 → 下游验证 → 主仓更新子模块指针和规则 → 主仓集成证据。

## 3. 批次 A：Paireads 紧急止损

优先级：P0，必须最先完成。

### A1. 输入输出身份校验

覆盖：PAIR-C01。

实施：

- 定义路径/文件身份检查函数，覆盖绝对路径、symlink 和 hardlink。
- Single mode 拒绝 input 与 output 相同。
- Dual mode 拒绝 R1、R2、两个输出和 read-name 文件之间的危险别名。
- 所有检查在创建任何输出前完成。

测试：

- 同字符串路径。
- 相对路径与绝对路径指向同一文件。
- symlink alias。
- hardlink alias。
- 每个用例断言非零错误，并比较输入前后 SHA-256。

### A2. 传播所有必需阶段错误

覆盖：PAIR-H01。

实施：

- 将 `runSingleBAMMode`、`runDualBAMMode` 改为返回 error。
- sort、index、writer close、rename 失败全部向上返回。
- `main` 统一打印错误并非零退出。
- 空输出定义为合法 BAM；sorter 必须能处理零记录，或在确认 header 已满足目标 sort order 后跳过重排。

测试：

- 零 complete pairs。
- 不可写输出目录。
- sort/index/rename 失败注入。
- `--help` 状态码为 0，未知 flag 为非零。

### A3. 原子输出

覆盖：PAIR-H05。

实施：

- 在目标目录使用 `os.CreateTemp`/唯一临时 BAM。
- 显式检查 writer close。
- 在临时路径完成 name/coordinate sort、BAI 和 samtools/bamdriver 验证。
- 使用 rename 安装最终 BAM/BAI。
- Dual mode 在两个输出都准备好后再提交；失败时清理整个批次临时产物。

完成门禁：PAIR-C01、PAIR-H01、PAIR-H05 的回归测试全部通过。

## 4. 批次 B：Paireads 底层和语义修复

### B1. 升级 bamdriver-go

覆盖：PAIR-H02，并降低 PAIR-H04 的排序风险。

实施：

- 升级到 `a22f777` 对应 pseudo-version或后续正式版本。
- `go mod tidy`。
- 删除不再需要的兼容空文件或保留薄 re-export，但不得复制 BAM 实现。
- CI 增加 malformed BAM/BGZF 和 samtools 区域查询。

### B2. 冻结配对领域模型

覆盖：PAIR-H03、PAIR-M01、PAIR-M02。

必须先明确三个不同概念：

- `CompletePrimaryMates`：同名、唯一 primary R1 + R2。
- `SAMProperPair`：满足 SAM flag/伴侣一致性要求。
- `MatchedNamesAcrossFiles`：两个文件的 read-name 交集。

实施选择：

- 主仓 Bismark single-BAM 路径预计需要 `CompletePrimaryMates`，用明确名称和帮助文本表达。
- Dual mode 若仅需要名称交集，应改名/改日志；若对外承诺 proper pair，则必须校验 mate flags、header 和 mate coordinates。
- 明确 retained fragment 是否写出 secondary/supplementary/unmapped records。

测试：

- 完整 primary pair。
- 非 proper 但完整 pair。
- 同名双 R1。
- mate coordinate/reference 不一致。
- secondary/supplementary records。
- 两个 BAM header 不一致。

### B3. 有界 read-name 处理

覆盖：PAIR-H04。

实施：

- Single mode 输入已由主仓 name sort，可按连续 read-name group 流式判断和写出。
- Dual mode要求输入 query-name sort，或先通过加固 bamdriver 外部排序到临时文件，再双流 merge。
- 删除全量 read-name set；filtered name list 采用流式有序写入。
- 提供 `--memory-limit` 或使用 bamdriver sort memory budget。

完成门禁：百万级 fixture 峰值内存满足书面预算，结果与小规模 reference implementation 一致。

## 5. 批次 C：xenofilter-go 状态、输入和产物修复

### C1. 失败状态传播

覆盖：XENO-H01。

实施：

- `runFilter` 聚合所有失败样本并返回 error。
- 保留逐样本表格，但有失败时退出非零。
- 主仓 Snakemake 的 success marker 只允许在命令成功后产生。

测试：

- 单样本失败。
- 多样本部分失败。
- 所有样本成功。
- Snakemake 最小规则不在失败时创建 marker。

### C2. 重排输入准备流程

覆盖：XENO-H02、XENO-M01、XENO-M02。

实施：

- 删除输入 BAI 要求，因为当前处理为顺序扫描。
- 验证 BAM 实际排序，而非只信 header。
- 未排序输入先进入独占临时排序文件。
- 验证 graft/host paired/single 模式一致，不再只查看第一条 graft record。

测试：

- 无 BAI 的已排序 BAM。
- header 与实际排序不一致。
- 未排序 BAM。
- graft/host 模式不一致。
- 只读输入目录。

### C3. 并发隔离和原子输出

覆盖：XENO-H03、XENO-H07。

实施：

- 每次命令创建私有 temp root，每样本创建独立目录。
- 校验 output names 唯一。
- BAM/BAI 在目标目录临时路径完成并验证后原子安装。
- context cancellation：任一严重失败后停止未开始任务，并安全结束运行中的任务。

测试：

- 两个不同目录的同 basename BAM 并行。
- 重复 output names。
- 两个进程并发。
- writer/index 失败不留下部分最终产物。

## 6. 批次 D：xenofilter-go 科学正确性

### D1. Bisulfite/NM 语义

覆盖：XENO-H05。

实施：

- `--bisulfite` 自动强制 reference recalculation，或配置验证要求显式 `--recalculate-nm`。
- 统一 `Calculate` 和 reference-aware 路径对 NM、insertion、soft/hard clip 的定义。
- 对照原 XenofilteR/R 实现冻结 scoring table。

### D2. Paired score 精确比较

覆盖：XENO-H06。

实施：

- 直接比较 mate score 总和，避免整数平均截断。
- 覆盖缺失 mate/unmapped penalty。

### D3. Threshold 契约

覆盖：XENO-M03。

实施：

- 用原实现或冻结数据确认 threshold 是 inclusive 还是 exclusive。
- 同步 CLI help、README、变量名和测试。

科学门禁：

- 普通 single/paired。
- bisulfite C→T/G→A。
- insertion/deletion/soft clip。
- score 总和相差 1。
- threshold-1/threshold/threshold+1。
- 与参考实现逐 fragment 分类一致。

## 7. 批次 E：xenofilter-go 有界内存架构

覆盖：XENO-H04、XENO-M04。

实施：

- 将 graft/host BAM 外部 query-name sort 到独占临时路径。
- 设计双流 merge，以 fragment 为最小工作单元分类并立即写入中间结果。
- 输出最终要求 coordinate sort 时，使用加固 bamdriver 的有界外部 sort。
- 分类统计增量计算，不保留全量 classification map。
- 删除 `CalculateBatch`，或改成固定 worker pool。
- 把 sample 并发数与单样本内存预算联合限制。

完成门禁：大 fixture 峰值内存有明确上限，1/2/4 workers 结果一致。

## 8. 批次 F：CI、主仓集成和发布证据

### 子仓 CI

两个仓库均增加：

```bash
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
govulncheck ./...
```

并保留/扩展 samtools 1.x 兼容门禁：

- `quickcheck`
- `view -c`
- header 读取
- sort order 检查
- BAI 区域查询

### 主仓集成

- `Paireads`：普通和 PDX Bismark extractor 最小 fixture。
- `xenofilter-go`：PDX 普通模式与 bisulfite 模式 fixture。
- 失败注入：子工具非零退出时 Snakemake 不创建 success marker。
- `scripts/build-all-submodules.sh`、`scripts/install.sh`、工具版本检查通过。
- 更新两个子模块指针后运行主仓相关 Go 测试和 dry-run。

## 9. 建议提交拆分

按以下顺序提交，避免一个巨大不可审查提交：

1. `Paireads`: test(path-safety) — 添加同路径/alias 失败测试。
2. `Paireads`: fix(path-safety) — 拒绝别名并保护输入。
3. `Paireads`: fix(atomic-output) — 错误传播和原子输出。
4. `Paireads`: chore(bamdriver) — 升级依赖和兼容门禁。
5. `Paireads`: refactor(pair-model) — 冻结配对语义。
6. `Paireads`: refactor(streaming) — 有界 name-group 处理。
7. `xenofilter-go`: test(failure-contract) — 失败退出和未排序输入测试。
8. `xenofilter-go`: fix(execution) — 状态传播、输入流程和临时隔离。
9. `xenofilter-go`: fix(scoring) — bisulfite、paired sum、threshold。
10. `xenofilter-go`: refactor(streaming) — 有界双流分类。
11. 主仓：更新规则、子模块指针和集成证据。

提交消息遵循仓库 Conventional Commit 风格。每个提交在推送前运行对应子仓完整门禁。

## 10. Wave 1 完成定义

- [x] PAIR-C01 清零并有文件哈希回归测试。
- [x] 两个仓库 Critical/High 均为 0。
- [x] 所有 Medium 已修复或有负责人、理由和明确发布门禁。
- [x] `Paireads` 使用已审查 `bamdriver-go`。
- [x] 两个工具的错误路径均非零退出且不产生部分最终产物。
- [x] 配对、NM、bisulfite、threshold 语义有 reference/golden tests。
- [x] 大输入处理使用显式有界内存架构。
- [x] Go test/vet/race/vuln 和 samtools 门禁进入 CI 并通过本地验证。
- [ ] 主仓普通、PDX、甲基化三条相关路径集成通过；当前环境缺少 Bismark/Snakemake 工具链。
- [x] 子仓提交已推送，主仓子模块指针和审查进度纳入本次发布提交。
