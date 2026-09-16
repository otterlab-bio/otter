# htseq2matrix-go 系统审查报告（2026-07-22）

## 1. 审查结论

- **仓库基线**：`a888938cab2ae78fc5c135b207ebf3f4f1ff68dc`
- **分支**：`master`
- **审查状态**：只读审查完成，发现 2 Critical / 6 High / 7 Medium / 1 Low
- **发布结论**：**阻断发布**。机械质量门禁全部通过，但科学数据完整性与参考兼容性门禁未通过。

## 2. 门禁结果

| 命令 | 结果 |
|---|---|
| `go test -count=1 ./...` | 通过 |
| `go vet ./...` | 通过 |
| `go test -race -count=1 ./...` | 通过 |
| `govulncheck` | 通过（`No vulnerabilities found.`） |
| `CGO_ENABLED=0 go build` | 通过（静态二进制） |
| R 依赖可用 | 通过 |
| Go/R 相同 fixture 差分 | **失败** |
| 输出故障传播（`/dev/full`） | **失败** |
| 两文件事务性 | **失败** |
| 非法计数拒绝 | **失败** |
| 数据库可复现性 | **失败** |
| 样本名冲突 | **失败** |

测试覆盖率：`cmd/htseq2matrix`、`internal/database`、`internal/output` 均为 0%；`internal/htseq` 72.2%；`internal/processor` 84.2%；`pkg/dataframe` 95.3%。`TestParseHTSeqLine` 只比较常量未调真实解析器，`TestIsCommentOrSummary` 无断言。

## 3. 数据流

`filepath.Glob` 按词典序遍历 → 逐行 trim、跳过空行/`#`/`__` 行 → 要求两列 tab → `ParseFloat` → 拒绝单样本内重复 gene ID → 所有样本 gene ID 做 **union**，缺失填 0 → 嵌入库精确字符串查询，未映射 ID 原样保留 → 重复 symbol 按列取 max → 行和过滤零/全 NaN/含 `-Inf` → 直接写 `matrix_count.txt` → `log2(x+1)` → 直接写 `matrix_norm.txt`。

## 4. Critical 发现

### HTSEQ-C01：Git remote URL 包含明文凭据

- **范围**：主仓与子仓 `.git/config` 的 `remote.origin.url`，匹配 GitHub token 格式。
- **整改**：立即吊销/轮换该 token，改无凭据 URL，排查日志/CI 暴露范围。（与 methrix-cli 审查独立确认）

### HTSEQ-C02：输出缓冲区错误被静默吞掉，程序可虚假报告成功

- **位置**：`pkg/dataframe/dataframe.go:132-163`。
- **证据**：`defer writer.Flush()` 但从不检查 `writer.Error()` 与 `file.Close()` 错误；`/dev/full` 动态测试退出码 0 且打印成功。
- **主仓放大**：`inst/rules/rnaseq_step2_checker.smk:1-18` 只依赖文件存在并 `touch` 成功标志，静默写失败可能被整个流程判为成功。
- **整改**：显式 `Flush()` 后检查 `writer.Error()`、`Sync()`、`Close()`；传播至 CLI 非零退出；加 `/dev/full` 故障注入测试。

## 5. High 发现

### HTSEQ-H01：Go 与原 R 的合并、缺失和未映射语义不兼容

- **位置**：Go `internal/processor/merger.go:11-68`、`internal/processor/converter.go:35-49`；R `inst/Rscripts/htseq2matrix.R:20-83`。
- **证据**：Go 做 union、缺失填 0、未映射 ID 保留；R 做首样本 left join、缺失保留 `NA`、未映射被 right join 丢弃。`merger.go:11-12` 注释声称 left join 但实现为 union。
- **差分证据**：`A2MP1` 只在第二样本时 Go 输出 `0,4`、R 不输出；`NAT1` 只在首样本时 Go `9,0`、R `9,NA`；一对多 `ENSG00000156273` Go 仅 `GRIK1-AS2`、R 输出 `BACH1`+`GRIK1-AS2`。
- **整改**：冻结矩阵合并契约；若替代 R，必须实现首样本 left join、NA 缺失、映射 right join。

### HTSEQ-H02：一对多 gene ID 映射被 map last-wins 破坏

- **位置**：`internal/database/embed.go:72-84`、`internal/database/csv.go:78-96`、`internal/processor/converter.go:35-49`。
- **证据**：同一输入 ID 关联多 symbol 时只保留 CSV 最后出现的；Human 667 个 ID 对应多 symbol（最多 211），Mouse 992 个（最多 101）；`ENSG00000156273` 同时映射 `BACH1`+`GRIK1-AS2` 但只输出后者。
- **整改**：改 `map[string][]Mapping`；明确一对多策略（展开/筛选/拒绝）；构建期校验冲突。

### HTSEQ-H03：负数、分数、NaN、Inf 被当作合法 HTSeq count

- **位置**：`internal/htseq/reader.go:78-99`、`pkg/dataframe/dataframe.go:32-64,103-110,112-128`。
- **证据**：`-1`→count `-1`/norm `-Inf`；`-2`→norm `NaN`；`1.5` 正常输出；`Inf` 保留；`IsRowValid` 只排除 `-Inf` 不排除 `+Inf`；退出码仍 0。
- **整改**：严格非负整数解析；拒绝小数/符号数/NaN/Inf；finite 校验。

### HTSEQ-H04：两文件输出非原子，可形成新旧矩阵混合

- **位置**：`internal/output/writer.go:11-31`、`pkg/dataframe/dataframe.go:132-163`、`cmd/htseq2matrix/main.go:121-136`。
- **证据**：count 写成功后 norm 失败时，`matrix_count.txt` 已被新矩阵替换而 norm 仍旧；`os.Create` 跟随预存符号链接。
- **整改**：同文件系统临时文件，flush/sync/close 后原子 rename；`Lstat` 拒绝非普通文件。

### HTSEQ-H05：小鼠数据库命名空间与主仓 HTSeq 路径不匹配

- **位置**：主仓 `inst/rules/rnaseq_htseq.smk:1-17`、`internal/database/embed.go:110-119`。
- **证据**：mouse CSV 来自 RDA 的 UNIPROT 列，`ENSMUSG...` ID 数为 0；主仓默认 GTF 为 `GRCm38.ensGene`，HTSeq `-i gene_id` 输出 ENSEMBL ID。
- **整改**：按主仓实际 GTF 冻结输入命名空间；小鼠提供 ENSEMBL→SYMBOL；加 `--id-type`。

### HTSEQ-H06：嵌入数据库来源、版本和生成流程不可复现

- **位置**：`convert_rda_to_csv.R:35-52`、`internal/database/embed.go:11-12`。
- **证据**：无来源/版本/许可证/manifest；当前脚本生成 `ENSEMBL+SYMBOL` 的 mouse CSV，无法重建当前嵌入的 `UNIPROT+SYMBOL` CSV；Mouse RDA 90,754 缺失 UNIPROT 被序列化成字面量 `NA` 并 last-wins 覆盖。
- **整改**：数据库 manifest（来源/release/哈希/生成器版本）；生成脚本逐字节重建当前 CSV；跳过缺失 ID。

## 6. Medium 发现

- **HTSEQ-M01**：不支持 ENSEMBL 版本后缀（Human CSV 版本化 ID 数为 0，无剥离逻辑）。
- **HTSEQ-M02**：样本名无校验（空名、`Gene` 保留名冲突、tab/换行均通过；动态测试生成表头 `Gene Gene normal`）。
- **HTSEQ-M03**：`float64` 丢失超大整数 count 精度（`>2^53` 失真，`1e308` 接受）。
- **HTSEQ-M04**：重复 symbol 按 max 聚合丢弃计数（与 R 一致但科学合理性未记录）。
- **HTSEQ-M05**：输出格式、主仓手册和 CLI 契约矛盾（手册用不存在的 `--input-dir/--output`；称 TPM/RPKM 但实际 `log2(count+1)`；Go tab 分隔 vs R 空格；Go CSV quoting vs R `quote=FALSE`）。
- **HTSEQ-M06**：CI 与测试不能证明科学兼容（release CI 仅 `-short`；CLI/数据库/output 覆盖率 0%；无 R 差分；无 vet/race/govulncheck）。
- **HTSEQ-M07**：主仓物种自动推导不足（`DetectSpecies` 只查字面 `human`/`mouse`，`hg38`/`mm10` 等返回 `unknown`）。

## 7. Low 发现

**HTSEQ-L01**：特殊行匹配过宽（所有 `__*` 和 `#*` 无条件丢弃，不限于标准 summary；解析错误无行号）。

## 8. R 参考兼容性矩阵

| 契约 | 结果 |
|---|---|
| 正常正有限已映射 human count | 部分通过 |
| gene universe | 失败（union vs left join） |
| 缺失填充 | 失败（0 vs NA） |
| 未映射 ID | 失败（保留 vs 丢弃） |
| 一对多映射 | 失败（last-wins vs 展开） |
| Mouse ENSEMBL | 失败（无映射） |
| 文本格式 | 失败（tab vs 空格） |
| quoting | 失败（CSV quoting vs `quote=FALSE`） |
| 行顺序 | 失败 |

## 9. 完成门禁缺口

- [ ] Git remote 凭据轮换。
- [ ] `csv.Writer.Error()`、sync、close 检查。
- [ ] 两矩阵事务性/原子发布。
- [ ] 冻结并实现 merge/missing/unmapped 语义。
- [ ] Go/R 逐单元格差分门禁。
- [ ] 严格拒绝负数/分数/NaN/Inf。
- [ ] 原始 count 改精确整数类型。
- [ ] 一对多映射禁 last-wins。
- [ ] 统一 mouse 命名空间与数据库。
- [ ] 数据库 manifest/版本/哈希。
- [ ] 修正主仓手册 CLI flags 与 TPM/RPKM 描述。
- [ ] vet/race/govulncheck/输出故障测试纳入发布 CI。
