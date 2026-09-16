# gomats 系统审查报告（2026-07-22）

## 1. 审查结论

- **仓库基线**：`38b96271782c32c2526d2afadef7d661eda91c5f`
- **分支**：`main`，工作树干净，与 `origin/main` 一致
- **审查状态**：只读审查完成，发现 1 Critical / 8 High / 7 Medium / 4 Low
- **发布结论**：**阻断发布**。主仓 Snakemake 边界存在 shell 注入、无 dry-run、部分失败与零任务可被报告为成功、缺失完整一致性校验与产物完整性校验。

## 2. 门禁结果

| 命令 | 结果 |
|---|---|
| `go test -count=1 ./...` | 通过 |
| `go vet ./...` | 通过 |
| `go test -race -count=1 ./...` | 通过 |
| `govulncheck`（v1.6.0，DB 2026-07-17） | 通过（`No vulnerabilities found`） |
| 真实 rMATS/主仓集成 | **未执行**（本地无 `gomats`/`rmats.py`/`enva` 可执行文件） |

覆盖率：`internal/bam` 83.8%、`internal/combinator` 100%、`internal/runner` 76.9%、`internal/seqkit` 83.3%、`pkg/cli` 19.7%、`internal/pdata` **0%**、`internal/types` 无测试。`internal/runner/runner_test.go:70` 忽略 `Run` 错误且后续断言只在文件不存在时记日志。

## 3. 数据流

`gomats run` → `pdata.Load` → `bam.Scan` → `seqkit.ComputeReadLength` → `combinator.Pairwise` → species×group 组合 → `runner.Run` → `rmats.py`（`exec.Command("rmats.py", args...)`，子进程本身安全）。

CLI flags：`--root`、`--threads`、`--pdata`、`--seqlengthQC`、`--gtf`、`--pdxmode`。无 `--dry-run`、无执行引擎选择、无任务并发控制、无恢复/跳过选项。

## 4. Critical 发现

### GOMATS-C01：主仓 Snakemake 路径未 shell 转义，可导致命令注入

- **位置**：`inst/rules/rnaseq_splicing.smk:34-41`。
- **证据**：规则直接使用 `--root {params.run_dir}`、`--pdata {params.pdata}`、`--seqlengthQC {params.seqlengthQC}`、`--gtf {params.gtf}`，无 `{params.x:q}` 或安全 argv。`gomats` 内部 `exec.Command` 安全，风险在主仓 Snakemake 边界。
- **影响**：含空格路径拆成多参数；shell 元字符（`;`、`$()`、反引号）执行额外命令。
- **整改**：统一 shell quoting，最好包装成不经 shell 的 argv 入口并对路径做存在性/类型/允许根目录校验。

## 5. High 发现

### GOMATS-H01：主仓支持 `condition` 分组，但 gomats 只接受 `sample_group`

- **位置**：主仓 `cmd/create.go:535`；gomats `internal/pdata/pdata.go:60,72`。
- **影响**：pdata 只有 `condition`/`treatment` 时，父仓进入 RNA splicing，gomats 直接报缺 `sample_group`，主仓支持路径确定性失败。
- **整改**：统一 pdata 分组字段契约或按优先级归一化。

### GOMATS-H02：重复 sample ID 静默覆盖，缺失字段行静默丢弃

- **位置**：`internal/pdata/pdata.go:76-86`。
- **影响**：后出现重复 sample 覆盖前者；缺列行直接跳过；空分组仍进 map 可能生成空组 contrast；父仓 CSV parser 拒绝重复 ID 但 gomats Excel 入口无此保证。
- **整改**：重复/缺失/空分组带行号报错，不得用 map 直接覆盖。

### GOMATS-H03：缺失或未匹配 BAM 只警告并跳过，未验证完整样本集合

- **位置**：`internal/bam/scanner.go:32,72-81`。
- **影响**：可能在缺失/重复 replicate 或不完整 cohort 上继续执行；rMATS 成功不代表样本集合正确。
- **整改**：建立 expected/observed 差集，缺 BAM/重复/不在 pdata 均报错。

### GOMATS-H04：PDX/多物种扫描可能使用错误 GTF

- **位置**：`internal/bam/scanner.go:18`、`pkg/cli/run.go:97`、`inst/rules/rnaseq_splicing.smk:24`、`internal/runner/runner.go:51`。
- **影响**：`Filtered_bams/` 多物种 BAM 或遗留 BAM 被全扫描，但所有任务复用同一 graft GTF，产生错误注释结果。
- **整改**：允许物种集合、每物种 BAM/GTF 显式配置；扫描限定在 expected inputs。

### GOMATS-H05：零 combination 时返回成功并可能写入 DONE 标记

- **位置**：`pkg/cli/run.go:100,110,158-161`、`internal/combinator/combinator.go:24`、`inst/rules/rnaseq_splicing.smk:41`。
- **影响**：单组/空 group/有效 BAM 全被过滤时，任务数为 0 仍返回 nil，主仓写 `RNASplicing_DONE`，造成"成功但无分析"。
- **整改**：计划显式要求物种≥1、组≥2、每组每物种≥1 BAM、组合>0。

### GOMATS-H06：分组名可造成输出路径穿越和输出冲突

- **位置**：`pkg/cli/run.go:52-53,133`、`internal/runner/runner.go:35,41`。
- **影响**：分组值含 `/`、`..`、绝对路径可逃逸 `root/RNASplicing`；含 `_vs_` 的组名可致不同 contrast 同目录；`b1.txt`/`b2.txt` 用 `os.Create` 覆盖。
- **整改**：用户分组值不作路径；用安全 ID/编号 + 元数据；验证最终路径位于 root 下。

### GOMATS-H07：部分失败后保留脏输出，重跑非幂等

- **位置**：`pkg/cli/run.go:109-146`、`internal/runner/runner.go:35,41-44`。
- **影响**：中间任务失败后部分 rMATS/`.rmats` 保留，后续任务继续，最终只返回汇总错误；重跑复用或覆盖旧目录；无输入/任务 manifest。
- **整改**：每 contrast 用 staging 目录，成功后原子发布，失败删除/隔离；写输入摘要、命令、版本、产物清单。

### GOMATS-H08：只检查 rmats.py 退出码，不检查产物完整性

- **位置**：`internal/runner/runner.go:65-70`。
- **影响**：退出码 0 但输出缺关键文件/空文件时，gomats 与主仓成功标记均判成功。
- **整改**：按锁定 rMATS 版本建立产物契约，验证关键 AS 类型文件非空可读。

## 6. Medium 发现

- **GOMATS-M01**：N50/readLength 缺有限性/范围/样本来源校验（`ParseFloat` 接受 NaN/Inf/负数/零，所有 `_val_` 行进均值）。
- **GOMATS-M02**：BAM 路径含逗号/换行会破坏 rMATS b1/b2 输入格式。
- **GOMATS-M03**：无 gomats dry-run，主仓 dry-run 也不验证完整计划。
- **GOMATS-M04**：无本地/Slurm 并发模型或 per-task 状态（`--threads` 只传给单次 rMATS）。
- **GOMATS-M05**：子进程退出码被压平为通用 1（OOM/信号/参数错误不可区分）。
- **GOMATS-M06**：rMATS 版本只做 `--help` 预检，无运行时版本/兼容门禁（环境固定 `rmats=4.1.2`，最新 v4.3.0）。
- **GOMATS-M07**：Excel 解析边界较弱（固定 sheetMap[1]；alias 在 trim/lower 前查找；大小写/空白/BOM 处理不一致；`excelize v1.4.1` 老版本；`internal/pdata` 覆盖率 0%）。

## 7. Low 发现

- **GOMATS-L01**：sample 顺序未按 pdata 保留（map + `filepath.Glob` 词典序）。
- **GOMATS-L02**：CLI 数值/模式参数校验不足（`--threads` 允许零负；`--pdxmode` 仅 `"1"` 为 true；`MarkFlagRequired` 返回错误未检查）。
- **GOMATS-L03**：文件 Close 错误未传播（`defer f.Close()`）。
- **GOMATS-L04**：Makefile 本地构建不注入版本（默认 `0.1.0`）。

## 8. 主仓实际调用检查

`inst/rules/rnaseq_splicing.smk`：输入只声明 BAM；pdata/seqkit QC/GTF 为 `params` 非 `input`；输出只有 `RNASplicing_success.txt`；gomats 任务目录/b1/b2/rMATS 产物未声明。pdata/GTF/QC 改变但 BAM 不变时，Snakemake 可能认为已完成不重跑。`RNASplicing_DONE` 只反映进程返回 0。

## 9. 安全观察（范围外但紧急）

主仓 `git remote` 输出中包含嵌入式 PAT 凭据（未在本报告回显）。应立即轮换并改用 SSH 或 credential helper。与 methrix-cli、htseq2matrix-go 审查一致确认。

## 10. 完成门禁缺口

- [ ] Critical/High 风险为零。
- [ ] pdata/Excel 核心边界有 fuzz/property 测试。
- [ ] fake `rmats.py` 捕获所有组合与参数数组。
- [ ] 单组/双组/多组/重复样本/空组/缺失 BAM fixtures。
- [ ] 真实 rMATS dry-run 或集成运行。
- [ ] 本地/Slurm 并发与部分失败证据。
- [ ] 重跑幂等与失败清理证据。
- [ ] 版本差异与产物完整性证据。
## 11. 已实施整改（2026-07-23）

- pdata 重复、空字段和 `sample_group`/`condition` 回退已严格校验。
- expected/observed BAM 集合、重复 sample/species BAM 和 pdata 外样本已 fail closed。
- 用户分组名不再直接进入路径；零 combination、无效线程和 PDX 参数返回错误。
- 主仓 `rnaseq_splicing.smk` 已对路径使用 `{value:q}`，pdata/GTF 纳入显式 input。
- runner 已改为同级 staging 执行，写入 `gomats.contrast-manifest/v1`，验证五类 `*.MATS.JC.txt` 的文件类型、非空状态及 `ID/GeneID/FDR` 表头，然后使用备份和 rename 原子发布。
- fake `rmats.py` 回归覆盖成功、命令失败、缺产物、坏表头、重跑替换、BAM 分隔符和 symlink 最终路径。
- `go test -count=1 ./...`、`go vet ./...`、`go test -race -count=1 ./...`、`CGO_ENABLED=0 go build ./...` 与 `git diff --check` 通过。

仍阻塞：锁定 rMATS 4.1.2 的真实表头/产物验证、PDX species→BAM→GTF 显式契约和主仓真实集成。
