# xenofilter-go 系统审查报告（2026-07-21）

## 1. 审查结论

- **仓库基线**：`7eb40d2465016413d1bedef5c75403f40cbbbd91`
- **分支**：`master`
- **审查状态**：代码整改与本地动态门禁通过，整改提交 `6ae7f84` 已推送，主仓子模块指针纳入本次发布提交
- **发布结论**：**代码级阻塞和子仓发布已完成；真实 PDX/Snakemake 集成仍受当前工具链缺失阻塞**
- **已确认问题**：0 Critical、0 个未解决 High、0 个未解决 Medium；7 个 High 和 4 个 Medium 已修复

当前实现已修正科学评分、CLI 失败状态传播、未排序输入处理、输出冲突与产物发布安全，并完成有界内存架构：两份输入通过受预算约束的外部 query-name sort 后按名称组双流归并，分类和统计按 fragment 增量完成，graft 结果再经受预算约束的 coordinate sort 发布。样本并发和单样本排序预算共享 256 MiB 总预算，paired/single 模式会在组内、单侧 BAM 全局及 graft/host 两侧之间验证一致；未使用且会按记录创建 goroutine 的 `CalculateBatch` 已删除。已知 Critical/High/Medium 代码问题清零。

## 2. 范围和数据流

主要数据流：

```text
CLI flags
  → config.Config
  → filter.FilterParallel（并发数受 256 MiB 总排序预算约束）
  → graft/host 分别执行有界外部 query-name sort
  → 按连续 read-name group 双流归并
  → 验证 paired/single 模式一致性
  → 按 fragment 调用 classifier 并增量更新统计
  → graft 分类记录写入 query-name 中间 BAM
  → 有界外部 coordinate sort
  → 构建 BAI并以可回滚事务发布 BAM/BAI
```

重点审查路径：

- `pkg/cli/run.go`
- `internal/filter/filter.go`
- `internal/classifier/single_end.go`
- `internal/classifier/paired_end.go`
- `internal/classifier/edit_distance.go`
- `internal/config/config.go`
- `internal/bamnative/` 与 `internal/bgzip/` 对 `bamdriver-go` 的兼容封装
- `.github/workflows/ci.yml`
- 主仓 `inst/rules/XenofilteR.smk`

## 3. 已执行证据

### 3.1 Go 门禁

以下命令通过：

```bash
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
```

CI 已纳入 `go test -count=1 ./...`、`go vet ./...`、`go test -race -count=1 ./...` 和 `govulncheck ./...`；主仓 CI 的 xenofilter 子模块步骤同步执行相同门禁。

### 3.2 bamdriver-go 集成

当前依赖：

```text
github.com/otterlab-bio/bamdriver-go
v0.1.2-0.20260721055359-a22f77784fc4
```

已使用该依赖完成下游测试。

### 3.3 samtools 兼容性

使用 `samtools 1.24` 对仓库真实 fixture 执行：

- 输入 BAM `samtools quickcheck`。
- `xenofilter run` 单线程处理。
- 输出 BAM 和 `.bai` 存在。
- 输出 BAM `samtools quickcheck` 通过。
- `samtools view -c` 可读取 1142 条输出 alignment。
- 使用输出 header 中真实参考名 `1` 执行索引区域查询通过。

这证明正常 fixture 的输出 BAM/BAI 可消费，但不能覆盖下述错误路径和科学计算风险。

### 3.4 依赖漏洞检查

本机安装 `golang.org/x/vuln/cmd/govulncheck@latest` 后执行：

```bash
govulncheck ./...
```

结果为 `No vulnerabilities found.`；该门禁也已加入子仓和主仓 CI。

## 4. High 发现

### XENO-H01（已修复）：样本处理失败后 CLI 仍以状态码 0 退出

- **位置**：`pkg/cli/run.go:157-189`
- **证据**：`FilterParallel` 返回的 `SampleResult.Error` 只被打印；遍历结束后 `runFilter` 无条件 `return nil`。
- **动态复现**：传入不存在的 graft/host BAM，终端打印 `ERROR`，实际进程 `exit_status=0`。
- **影响**：主仓 `inst/rules/XenofilteR.smk` 在命令后直接 `touch filtered_success.txt`。任一样本失败时，Snakemake 仍可能创建成功标记并继续后续分析，属于工作流成功状态失真。
- **整改**：聚合失败样本；任一失败时返回非 nil error。保留逐样本错误摘要，但不得成功退出。成功标记应由 Snakemake 仅在命令状态码为 0 时创建。
- **回归测试**：一个缺失输入和一个多样本部分失败测试，断言 CLI 非零退出且不产生成功标记。

### XENO-H02（已修复）：未排序 BAM 在排序前被强制建索引，自动排序路径不可达

- **位置**：`internal/filter/filter.go:52-121`
- **证据**：代码先调用 `IsSorted`，但不论结果如何都在 `:66-80` 对原 BAM 执行 `BuildIndex`；排序发生在 `:92-125`。
- **动态复现**：构造 header 声明 coordinate、实际记录坐标逆序且无 BAI 的 BAM。程序先尝试建索引并失败：`cannot index BAM: records are not coordinate-sorted`，随后仍由 XENO-H01 以 0 退出。
- **影响**：README 所述自动排序不能处理最需要排序的输入；合法但未排序的 BAM 被拒绝。
- **整改**：删除顺序扫描前不必要的输入建索引；先确定实际排序状态，必要时排序到独占临时文件，再仅对最终输出构建索引。
- **回归测试**：未排序、错误 `SO` header、无输入 BAI 三种 fixture，断言成功生成排序后的输出 BAM/BAI。

### XENO-H03（已修复）：最终输出名冲突会导致并行样本覆盖

- **整改结果**：排序临时文件使用样本独占目录；批次启动前规范化并校验全部最终输出路径，拒绝重复路径、绝对输出名和 `../` 逃逸；每个最终目标使用 `O_EXCL` 锁文件阻止跨进程并发写。
- **回归测试**：CLI 重复输出和路径逃逸会在调用过滤器前失败；已有锁会阻止第二写入者。

### XENO-H04（已修复）：完整加载两份 BAM 和多套 read-name map，内存无上限

- **原位置**：旧版 `internal/filter/filter.go` 的全量 `[]*Record`、名称集合和 classification map。
- **整改结果**：graft/host 分别通过 bamdriver 的有界外部 query-name sort 写入独占临时路径；过滤器一次只保留当前两侧名称组并执行双流归并，分类结果和统计不再全量保存。选中的 graft 记录写入中间 BAM 后再执行有界 coordinate sort。
- **并发预算**：所有并发样本共享 256 MiB 排序内存预算，单 worker 最低 8 MiB，worker 数最高 32；`--threads` 超过该上限时不会继续放大排序内存。
- **回归测试**：10,000 个唯一名称的 fixture 验证 reader 每次仅返回一个名称组；内存预算表驱动测试验证 `workers × per-worker budget ≤ 256 MiB`；512 fragments 的顺序与并行过滤结果和输出完全一致。

### XENO-H05（已修复）：仅启用 `--bisulfite` 时，已有 NM tag 会绕过 bisulfite 重算

- **位置**：`internal/classifier/classifier.go:38-58`、`internal/classifier/edit_distance.go:59-90`
- **证据**：`IsBisulfite` 会启用 reference-aware classifier，但 `calculateWithReference` 在 `!recalculate && HasNM(...)` 时直接返回已有 NM，不使用 reference 和 bisulfite conversion 规则。
- **触发条件**：用户指定 `--bisulfite`，未指定 `--recalculate-nm`，BAM 已含 NM tag。
- **影响**：C→T/G→A conversion 仍按普通 mismatch 计分，命令行承诺的 bisulfite 模式被静默忽略，可能改变 host/graft 分类。
- **整改**：bisulfite 模式必须强制 reference-aware NM 计算，或者 CLI 明确要求并自动启用 recalculation；配置校验应禁止产生语义矛盾的组合。
- **回归测试**：同一条含 conversion 且带 NM tag 的 alignment，在普通模式和 bisulfite 模式下断言不同且符合人工结果。

### XENO-H06（已修复）：paired score 使用整数平均会把真实优劣压成平局

- **位置**：`internal/classifier/paired_end.go:64-73`、`internal/classifier/paired_end.go:145-154`
- **证据**：两个 mate score 先相加再做整数 `/ 2`。例如 graft 总分 6、host 总分 7，二者平均值均截断为 3，结果被错误标记为 discarded。
- **影响**：总 edit distance 相差 1 的 paired fragment 可能从 graft/host 变为 discard，直接改变科学结果。
- **整改**：不需要计算平均值；直接比较两个 mate score 的总和。若未来 mate 数不同，应使用精确有理数比较而不是整数截断。
- **回归测试**：覆盖总分相差 1、相等、含 unmapped penalty 的表驱动测试。

### XENO-H07（已修复）：最终 BAM 直接写入目标路径，失败会留下可见的部分产物

- **整改结果**：BAM 和 BAI 先写到最终目录内的随机暂存路径；writer close 和索引构建都成功后才发布。若替换既有产物，先备份旧 BAM/BAI；任一发布 rename 失败时删除不完整新文件并恢复旧文件，恢复错误会合并返回。
- **回归测试**：覆盖索引构建失败保持旧产物、BAI 发布失败恢复旧 BAM/BAI、暂存文件和锁文件清理。

## 5. Medium 发现

### XENO-M01（已修复）：输入 BAI 被修改但整个算法未使用随机访问

- **位置**：`internal/filter/filter.go:66-80`
- **影响**：只读数据目录或共享输入会因缺少写权限失败；运行会在输入旁产生额外文件。
- **整改**：移除输入索引要求，只有最终需要区域查询的输出才构建 BAI。

### XENO-M02（已修复）：paired/single 模式仅由第一条 graft record 决定

- **整改结果**：名称组进入分类前验证组内所有 primary mapped records 的 paired flag 一致；流式处理期间分别冻结 graft 和 host 的样本级模式，并验证两侧模式一致。任一组内混合、单侧 BAM 前后混合或 graft/host 不一致都会返回明确错误。
- **回归测试**：覆盖同一 graft BAM 跨名称混合模式，以及同名 graft paired / host single 的跨输入不一致。

### XENO-M03（已修复）：`MMThreshold` 的边界语义与“maximum mismatches”描述不一致

- **位置**：`internal/classifier/single_end.go:47,102`、`internal/classifier/paired_end.go:48,129`
- **证据**：描述为最大允许 mismatch，但实现以 `score >= threshold` 丢弃，意味着阈值本身不允许。
- **整改**：对照原 XenofilteR 冻结一个 oracle；若参数表示最大允许值，应改为 `score > threshold`，否则修改参数名和文档。
- **回归测试**：至少覆盖 `threshold-1`、`threshold`、`threshold+1`。

### XENO-M04（已修复）：未使用的 `CalculateBatch` 为每条记录创建一个 goroutine

- **整改结果**：仓库内没有调用方，该未使用 API 已删除，避免未来误用时按记录数创建 goroutine。

## 7. 已完成整改

本轮已完成：

- CLI 聚合所有失败样本；任一样本失败时返回包含失败数量、样本名和根因的非 nil error，使进程以非零状态退出。
- 添加全成功和多样本部分失败回归测试，确保成功结果仍会汇总，但部分失败不会被报告为成功。
- `--bisulfite` 在存在 NM tag 时也强制使用 reference-aware NM 重算。
- 统一已有 NM 和 reference 重算路径的 XenofilteR score 为 `NM + insertion length + soft-clip length`；hard clip 不计入。
- paired-end 比较改为比较两个 mate score 的总和，消除整数平均截断。
- 根据原始 XenofilteR R 源码和 README 冻结 threshold 规则：graft score 必须严格小于 `MM_threshold`；score 等于 threshold 时 discard。
- 添加 bisulfite、score 公式、hard clip、paired 总分和 threshold 边界回归测试。
- 删除输入 BAM 的 BAI 前置创建；顺序读取不再要求输入索引，也不会在输入目录旁写入 `.bai`。
- 未排序输入先通过逐记录检查识别，再排序到 `os.MkdirTemp` 创建的样本独占目录；graft/host 使用不同临时文件名。
- 添加 header 声明 coordinate、实际记录逆序、无 BAI、graft/host 同 basename 的过滤回归测试，验证输出 BAM/BAI 成功且输入旁不生成索引。
- 输出路径在批次启动前规范化，拒绝重复最终路径、绝对输出名和输出目录逃逸。
- 每个最终 BAM 使用排他锁文件阻止多个进程同时发布同一目标。
- BAM/BAI 改为同目录暂存并以带旧产物备份和失败恢复的发布事务提交。
- 添加重复输出、路径逃逸、并发锁、索引失败保持旧产物和 BAI 发布失败回滚测试。
- 将过滤流程重构为 query-name 外部排序、名称组双流归并、增量统计和最终 coordinate 外部排序，不再全量保存 records、名称集合或 classification map。
- 以 256 MiB 总预算联合约束样本并发和单样本排序内存，并添加预算上界、10,000 名称流式分组及顺序/并行输出等价测试。
- 验证组内、单侧 BAM 全局和 graft/host 两侧 paired/single 模式一致性；混合输入会明确失败。
- 删除未使用且会按记录创建 goroutine 的 `CalculateBatch`。
- 子仓和主仓 CI 已加入 test、vet、race、govulncheck；samtools 门禁增加 sort-order 和 BAI 区域查询。
- 主仓当前及 legacy XenofilteR 规则均使用 `&& touch`，并有资产测试防止失败命令生成 success marker。

本轮验证：

```bash
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
```

以上全部通过；samtools 1.24 真实 fixture 兼容测试通过，输出计数为 1142。

## 8. 测试覆盖缺口

`internal/filter` 已覆盖无索引逆序 BAM、流式名称组、模式一致性、预算上界和顺序/并行等价；`pkg/cli` 已覆盖全成功与多样本部分失败的状态传播；主仓资产测试已冻结 success marker 必须由成功命令通过 `&&` 创建。仍可继续增强但不对应当前已知 Critical/High/Medium 的项目包括：

- 更完整的 single-end 和 paired-end classification truth table。
- unmapped penalty 的全部组合边界。
- 只读输入目录和多进程锁的真实进程级验证。
- 特殊文件系统或磁盘故障下更深入的发布/恢复故障注入。
- 百万级 fixture 的独立进程 RSS 基准；当前已有结构性有界测试和 256 MiB 聚合预算断言。
- 与原 R XenofilteR 的完整 golden/differential test。
- 主仓真实 Snakemake PDX dry-run，而不仅是规则 success-marker 契约测试。

## 9. 整改顺序

1. **P0 状态正确性（已完成）**：XENO-H01 已修复，任一样本失败会使 CLI 非零退出。
2. **P0 输入流程（已完成）**：XENO-H02/M01 已修复，输入不再预建索引，逆序 BAM 会在独占临时目录中排序。
3. **P0 并发隔离与原子产物（已完成）**：XENO-H03/H07 已修复，重复输出和路径逃逸会被拒绝，同目标跨进程写入由锁保护，BAM/BAI 使用可回滚暂存发布。
4. **P0 科学正确性（已完成）**：XENO-H05/H06 和 threshold oracle 已修复并冻结。
5. **P1 架构（已完成）**：XENO-H04 已改为有界外部排序与名称组双流归并，样本并发和排序内存共享固定总预算。
6. **P1 输入模式（已完成）**：XENO-M02 已验证组内、单侧和跨 graft/host 的 paired/single 一致性。
7. **P1 API 清理（已完成）**：XENO-M04 未使用的无界 goroutine 批处理 API 已删除。
8. **P1 自动门禁（已完成）**：CI 已加入 vet、race、govulncheck、samtools sort-order 和区域查询；主仓 success marker 使用显式成功链。
9. **后续增强（非当前阻塞）**：完整 R differential test、真实 Snakemake PDX dry-run和独立进程百万级 RSS 基准。

## 10. 完成门禁

- [x] 0 Critical / 0 High。
- [x] 所有已知问题的回归测试进入工作树。
- [x] `go test -count=1 ./...`、`go vet ./...`、`go test -race -count=1 ./...` 通过。
- [x] `govulncheck ./...` 通过，结果为 `No vulnerabilities found.`。
- [x] samtools quickcheck、view count、sort-order 和区域查询门禁已验证并进入 CI。
- [x] 普通、bisulfite、paired sum 和 threshold 核心分类语义已有冻结回归测试。
- [x] 主仓规则明确以 `&& touch` 保护 success marker，并有资产契约测试。
- [x] 子仓改动已以 `6ae7f84` 提交并推送，主仓子模块指针纳入本次发布提交。
- [ ] 可选增强：真实 Snakemake PDX dry-run和完整 R differential test。
