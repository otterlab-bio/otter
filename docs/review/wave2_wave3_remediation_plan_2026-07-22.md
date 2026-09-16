# Wave 2-4 剩余问题与实施计划（2026-07-25）

## 1. 范围决策

本计划以当前 `xdxtools` 原生工具链为唯一发布契约，采用以下明确边界：

- `qctb` 不再兼容历史 R QC 表结构，不生成 `qc_summary.RDS`，不以 R/Rust 逐字段一致作为发布门禁。
- `qctb` 需要定义并版本化自己的 Excel/TSV 报告 schema，并与主仓当前配置和实际上游产物一致。
- `methrix-cli` 不再兼容 `loadHDF5SummarizedExperiment()` 或 methrix R loader。
- `methrix-cli` 正式采用版本化的自定义 HDF5 schema：`methrix-cli.custom-hdf5`。
- `methrix-cli` 的消费者契约由 Rust 端结构校验、`qctb` 读取和主仓工作流集成证明。
- `htseq2matrix-go` 仍需冻结科学矩阵语义；历史 R 实现可作为差异分析参考，但最终契约必须在 Go 项目内明确记录并由原生 golden fixtures 固定。
- 缺少外部工具时必须标记为环境阻塞，不以单元测试或 dry-run 冒充真实集成通过。

## 2. 当前状态总表

| 仓库 | 已完成整改 | 当前代码门禁 | 剩余发布阻塞 |
|---|---|---|---|
| `enva` | 路径/删除边界、argv 保留、symlink 拒绝、ownership fail-closed、force staging/rollback、版本化 journal、全入口崩溃恢复、prefix/cache 跨进程锁、cache marker、真实 solve-cleanup 跨进程并发门禁、fail-closed `CONDA_PREFIX` root 推导、canonical prefix identity 去重、同名环境 unique-or-ambiguous fail-closed 解析、run/activate/install/adopt/remove 显式 prefix 消歧、typed capability matrix、external remove 显式 prior adoption、rattler 默认后端、兼容后端仅发现已安装二进制、MatchSpec 边界、结构化退出码、rattler dry-run 真实 solve、native install staging/原子发布 | fmt/严格 clippy/test/locked check 通过（120 deterministic library + 4 network integration + 4 CLI） | 兼容后端真实 smoke test 可选；native install hard-link/relocation/大 prefix 性能边缘；Windows 延期 |
| `gomats` | pdata 严格解析、`condition` 回退、BAM 集合完整性、安全 contrast ID、零任务失败、主仓安全引用、staging/manifest/关键产物验证/原子发布 | Go test/vet/race/静态构建通过 | 真实 rMATS 集成、PDX 物种→GTF 输入契约 |
| `fastqc-rs` | malformed/truncated FASTQ fail-closed、IUPAC、碱基级质量统计、A-T/G-C、确定性排序、单一 parser、typed 资源上限、离线安全 HTML 生成、FastQC-compatible/SeqKit summary | locked fmt/check/严格 clippy/test/build/diff check 通过（23 unit + 4 CLI）；大输入预算和 FastQC 0.12.1/SeqKit 2.13.0/MultiQC 1.35/qctb 真实消费通过 | GitHub-hosted compatibility job 待远端实跑；commit/push 与主仓指针待授权 |
| `htseq2matrix-go` | 严格非负整数 count、精确整数边界、writer 错误传播、双文件事务发布、符号链接拒绝、故障注入 | Go test/vet/race/静态构建通过 | merge/missing/unmapped、一对多映射、mouse 命名空间、数据库 provenance、原生 golden fixtures |
| `qctb` | 主仓配置类型、graft 选择、Methrix 路径、缺样本报错、FQC 数值边界、原子输出、格式枚举、`Cargo.lock` 可跟踪 | fmt/clippy/test 通过 | 原生报告 schema 版本化、各模式 golden fixtures、其余 parser 严格校验、主仓真实集成、CI 锁文件门禁 |
| `methrix-cli` | 自定义 schema 元数据、多产物事务发布、annotation 行数预检、下载超时/大小限制/`.part` 安装 | fmt/clippy/test 通过 | 原生 schema validator/reader、流式分块写入、可信 genome manifest/hash、崩溃恢复、主仓/qctb 集成 |
| 主仓 | gomats/qctb Snakemake 参数安全引用，配置/pdata/GTF 纳入显式 input，静态契约测试 | Go test/vet 通过 | 真实 Snakemake、rMATS、Bismark 和各数据链 smoke test；最终子模块指针尚未更新 |

## 3. 剩余问题清单

### P0：当前门禁与安全阻塞

#### P0-1 `enva` 完整测试稳定通过（已完成）

- 已移除缓存测试对 `TMPDIR`、`USER`、`RATTLER_CACHE_DIR` 的进程级修改。
- 缓存根解析改为纯输入函数，缓存清理测试直接注入隔离临时根目录。
- 幂等删除仅忽略删除阶段竞争产生的 `NotFound`，其他 containment、类型和权限错误仍传播。
- fmt、严格 clippy、定向测试通过；默认并行完整测试连续三次通过；`git diff --check` 通过。

完成证据：当前 120 个确定性库测试、4 个显式网络集成测试与 4 个 CLI 测试全部通过。

#### P0-2 Git 凭据安全响应（需要用户账户侧操作）

- 本地无回显检测确认主仓、`gomats`、`htseq2matrix-go`、`methrix-cli` remote 含嵌入式凭据。
- 必须在 GitHub 侧轮换或吊销对应 PAT。
- 将 remote 改为 SSH 或无凭据 HTTPS URL。
- 检查终端日志、CI 日志和 Git 历史是否含凭据。

完成标准：旧凭据失效，当前 remote 不含明文凭据，暴露范围有记录。本轮不擅自修改 Git 配置。

#### P0-3 最终工作树卫生（当前检查完成，发布前需复核）

- 主仓及六个子仓已执行 `git status --short` 与 `git diff --check`。
- 未发现 `htseq2matrix` 裸二进制、旧 `enva target-*` 目录或 `.tmp` 遗留。
- 已统一 `methrix-cli` 本轮新增 Rust 行的 LF 换行，全部仓库 `git diff --check` 通过。
- 用户原有 `enva` 改动已保留，`qctb/Cargo.lock` 继续作为应跟踪文件保留。

完成标准：当前只剩计划内源码、测试、锁文件和文档改动；发布前再执行一次相同检查。

### P1：代码级发布阻塞

#### P1-1 `gomats` 任务事务与产物契约（代码与 fake rMATS 门禁已完成）

- 每个 contrast 在最终目录同级的唯一 staging 目录执行。
- `gomats.contrast-manifest/v1` 记录 BAM、group、species、GTF、read length、线程、rMATS 版本、argv 和预期产物。
- 校验 `SE/MXE/A5SS/A3SS/RI.MATS.JC.txt` 为非空普通文件，并至少包含 `ID`、`GeneID`、`FDR`。
- 校验通过后使用备份与 rename 原子发布；命令失败、缺产物、坏表头均保留旧输出并清理 staging。
- fake `rmats.py` 已覆盖成功、非零退出、退出 0 但缺产物、坏表头、重跑替换、BAM 分隔符和 symlink 最终路径。
- Go test、vet、race、静态构建和 `git diff --check` 全部通过。

剩余门禁：在锁定 rMATS 4.1.2 环境确认五类输出表头契约，并完成主仓/PDX 真实集成。

#### P1-2 `enva` 破坏性操作事务化与崩溃恢复（代码与回归门禁已完成）

- `create --force` 使用同文件系统 staging install → conda-meta/ownership 验证 → old backup → rename publish → cleanup。
- 每个 prefix 使用固定的版本化 JSONL journal；阶段记录以 append + `sync_all()` 持久化，尾部记录被进程中断时可回退到最后一个完整记录。
- create 在取得 prefix advisory lock 后先恢复遗留事务，再检查现有环境和执行新安装。
- recovery 只操作 parent 相同、transaction ID 和 allocation suffix 精确匹配的 staging/backup；journal 损坏、跨目录或状态矛盾时 fail closed。
- create/install/remove 使用 prefix 同级 advisory lock；solve/install/cache cleanup 使用同一 cache-root lock，避免清理与读写并发。
- ownership marker 原子写入；损坏 marker、不可读取 marker 和无法安全 adopt 均 fail closed。
- cache cleanup 仅允许带 `enva-rattler` marker 的 cache root，拒绝 root/home/workspace/current directory 及其祖先。
- 回归测试覆盖发布成功、安装后发布失败、正常放弃 staging、`final → backup` 后子进程退出、恢复幂等、journal 尾记录截断、parent sync 失败、backup cleanup 失败和恶意跨目录 journal。
- native install 使用 reflink/copy sibling staging 克隆现有 prefix，仅在 staging 执行 Installer；ownership 与 `conda-meta` 验证通过后复用 journal 原子替换 final。
- 真实 conda-forge 生命周期测试覆盖 create、solver 失败、Installer 完成后故障注入、成功安装 Python、最终 prefix 运行校验和 remove；失败时旧 sentinel/ownership/package 集合不变，事务产物全部清理。

- 真实 cache 并发测试使用 conda-forge solve 持有 `CacheUse`，并在独立子进程执行 cleanup；cleanup 在 solve 期间阻塞，释放后删除 `pkgs`/`repodata`/`run-exports`，cache ownership marker 保留。

- `CONDA_PREFIX` 通过 `CondaPrefixLayout` 分类：标准 `envs/<name>` 推导 root，显式 base 使用 prefix 本身，外部、自定义和相对 prefix 不参与 root 检测。
- canonical environment identity 与名称解析通过 `EnvironmentResolution` 统一：已存在 prefix 用 canonical path 去重，run/activate/install/adopt/remove 的名称路径只接受唯一匹配；不同物理 prefix 同名时 fail closed，install/remove 支持显式 `--prefix`，且歧义失败不写 ownership、不删除或 adopt 任何 prefix。
- `BackendCapabilities` 以 `Native` / `Delegated` / `Hybrid` / `Unsupported` 固定 rattler 与 CLI compatibility 的操作语义；create/install/adopt/remove/list/validate/run 在命令边界验证 capability。默认 rattler remove 不再隐式 adopt external prefix，name/prefix 拒绝路径均保持目录和 marker 不变。

当前门禁：fmt、严格 Clippy、完整 120 deterministic library + 4 network integration + 4 CLI、locked check 和主仓门禁均通过。

后续健壮性任务：native install 的 escaping symlink、hard-link 保留、无 reflink 时大 prefix 性能与更多 relocation 覆盖；兼容后端真实 smoke test 可选；Windows 兼容按当前范围决策延期。

#### P1-3 `enva` 可选兼容后端边界（通过删除自动安装能力收口）

- `enva` 的正式默认后端是 rattler；micromamba、mamba 和 conda 仅用于显式兼容模式和外部环境互操作。
- `enva` 不再下载、解包、更新或发布 micromamba，因此不再维护平台资产 URL、hash、下载临时文件或安装事务。
- micromamba 仅从 `ENVA_MICROMAMBA_PATH` 或 `PATH` 发现；候选必须是可执行普通文件，且 `--version` 健康检查成功。
- `ENVA_MICROMAMBA_PATH` 一旦设置即具有权威性：无效路径直接失败，不回退到 `PATH`。
- 直接构造兼容 manager 时的 `ENVA_PACKAGE_MANAGER` 请求，以及 CLI `--pm` 请求，在目标后端不可用时 fail closed，不再静默切换到其他工具。
- 删除直接 `reqwest` 和 `sha2` 依赖；`Cargo.lock` 继续跟踪以固定 CLI 的完整依赖闭包。

当前门禁：纯路径解析、健康检查和 typed capability matrix 测试已添加；fmt、严格 Clippy、120 个确定性库测试、4 个网络集成测试、4 个 CLI 测试、locked check 和 diff check 全部通过。

剩余门禁：兼容后端真实 smoke test 属于可选互操作验证，不阻塞 rattler 主发布链。

#### P1-4 `htseq2matrix-go` 科学矩阵契约

- 明确 gene universe、缺失值、未映射 ID、重复 gene ID 和输出行顺序。
- 将一对多 mapping 从 last-wins 改为显式策略：展开、聚合或拒绝；策略必须写入 schema/manifest。
- 将 mouse 数据库对齐主仓 GTF 的 ENSEMBL `gene_id` 命名空间。
- 数据库增加来源、release、生成脚本版本和 SHA-256 manifest。
- 使用原生 golden fixtures 固定 human/mouse、多样本缺失、一对多和版本后缀行为。

完成标准：同一输入逐字节稳定；所有非显然映射行为有文档和测试，不再依赖 CSV 顺序。

#### P1-5 `qctb` 原生报告契约

- 定义 `qctb.report` schema 名称和版本。
- 固定 Excel/TSV 列名、顺序、类型、舍入和缺失值表示。
- 为 RRBS、WGBS、RNA-seq、PDX 各建立至少一组原生 golden fixture。
- Bismark、STAR、Qualimap parser 增加重复字段、范围、分母和内部一致性校验。
- 校验 SID 非空、去重，并与配置和可用报告集合一致。
- 跟踪 `Cargo.lock`，CI 使用 `--locked`。

完成标准：四种模式 golden tests 通过；schema 变化必须显式升级版本。

#### P1-6 `methrix-cli` 原生 HDF5 契约

- 为 `methrix-cli.custom-hdf5/1.0.0` 定义必需 group、dataset、属性、类型、坐标和维度约束。
- 实现 Rust 原生 validator/reader，对刚写出的产物做 round-trip 校验。
- `qctb` 只消费正式声明的 QC/metadata 契约，不依赖 R 对象结构。
- 为零样本、空位点、极端 coverage、重复 CpG、截断输入和维度不一致建立负向测试。

完成标准：自定义 schema 可由 Rust validator 完整验证，主仓和 `qctb` 不依赖未声明字段。

### P2：可靠性、性能与兼容性

#### P2-1 `methrix-cli` 大规模处理

- Bismark 输入改为流式或分批处理。
- HDF5 使用 chunked batch write，避免完整转置副本。
- 约束 Rayon 线程池，使 `--threads` 覆盖完整处理阶段。
- 为典型 RRBS 和 WGBS 规模定义内存预算并增加基准测试。

完成标准：在约定规模下峰值内存不随“完整矩阵副本数量”成倍增长，多线程和单线程结果等价。

#### P2-2 `methrix-cli` genome provenance

- 固定 genome URL、release、大小和可信 SHA-256 manifest。
- 下载后校验，安装时保留来源和 hash metadata。
- 对未知 genome 禁止伪造或跳过校验；自定义 genome 要求用户明确提供来源/hash 或标记为 unverified。

完成标准：官方 genome 可复现；错误、截断或替换内容不能进入正式缓存。

#### P2-3 `fastqc-rs` 异常输入和资源上限（本地 closure 完成）

- malformed plain FASTQ、truncated gzip、FASTA、空 read、quality 缺失/长度不一致和非法 Phred+33 均结构化 fail closed。
- CLI 返回非零，完整分析成功前不创建 summary 目录，不发布部分报告。
- k-mer 由 `KmerLength` 限制为 1..=7；per-position histogram 上限 10,000；单 read 上限 10,000,000 bases；distinct read lengths 上限 100,000。
- HTML 生成删除 reqwest/CDN 抓取和网络 `unwrap()`；不可达代理下仍可生成报告。
- fixtures 覆盖 plain/gzip、CRLF、可变长度、低质量、IUPAC、空 read、malformed 与截断输入。
- 100,000×150 bp、`k=7` 本机 gate：28.847 秒、14,208 KiB peak RSS，低于 120 秒/512 MiB 预算；CI 使用 Python `subprocess + resource` 执行同等 gate。

剩余门禁：GitHub-hosted compatibility job 首次远端实跑；不再有本地代码级 P2-3 阻塞。

#### P2-4 `fastqc-rs` 外部格式验证（本地 closure 完成）

- SeqKit 2.13.0 对比 Q20、Q30、AvgQual、长度四分位、N50、GC、sum_n 和 sum_gap，约定字段逐项一致。
- Babraham FastQC 0.12.1 对比 Total Sequences、Sequence length、GC 和 per-base quality 核心字段；报告明确为兼容子集，不声明全部模块 parity。
- MultiQC 1.35 真实读取 summary 并生成 FastQC/general stats/source/HTML 产物。
- qctb 真实解析 run4 `fastqc_data.txt`，确认 200 reads、20,200 bases、长度 101、Q20 96、Q30 91。
- `>>Seqkit Statistics` 与紧贴或独立 `>>END_MODULE` 的消费者契约保持稳定。

剩余门禁：固定外部下载 URL 需在 GitHub-hosted runner 实际执行；本地外部兼容 closure 已完成。

### P3：主仓跨仓集成

#### P3-1 RNA splicing 数据链

- 使用主仓真实配置运行 Snakemake rule → `enva run` → `gomats` → fake/真实 rMATS。
- 覆盖路径空格、特殊字符、单组、多组、缺 BAM、部分任务失败和重跑。
- PDX 模式显式建立 species → BAM set → GTF 映射，禁止 graft GTF 误用于其他物种。

完成标准：成功 marker 仅在所有计划任务和产物验证通过后生成。

#### P3-2 QC 数据链

- RRBS/WGBS：FQC/Bismark/Methrix 当前产物 → `qctb` → versioned Excel/TSV。
- RNA-seq：FQC/STAR/Qualimap 当前产物 → `qctb`。
- PDX：验证 graft、species list 和样本集合。
- 覆盖路径空格、报告缺失、报告存在但缺样本和重复样本。

完成标准：四种模式至少各一条主仓 smoke test，输出通过 `qctb` schema golden 校验。

#### P3-3 甲基化数据链

- 小型 Bismark coverage → `methrix-cli` → custom HDF5 validator → QC/annotation → `qctb`。
- 验证主仓声明全部 side effects，输入变化可触发重跑，失败不保留混合版本产物。

完成标准：不依赖任何 R loader，即可从输入到 QC 汇总完整验证数据维度、坐标、样本名和关键统计。

#### P3-4 RNA count 数据链

- HTSeq counts → `htseq2matrix-go` → count/norm matrix。
- 覆盖 human、mouse、未映射、一对多、缺失 gene 和写入失败。

完成标准：输出与冻结的原生 matrix schema/golden fixture 一致。

### P4：发布与交付

- 每个子仓独立提交并推送，提交信息遵循 Conventional Commits。
- 按依赖顺序更新主仓子模块指针。
- 运行 `scripts/build-all-submodules.sh` 和工具链一致性检查。
- 运行主仓 Go test/vet、相关工作流 smoke tests 和最终 `git diff --check`。
- 更新六份审查报告、`docs/active_context.md` 和总进度表。
- 未完成的 Medium/Low 风险必须有书面延期原因和后续门禁。

完成标准：从干净 clone 可重复构建，当前原生契约下 Critical/High 为零，主仓四条关键数据链均有可复核证据。

## 4. 推荐实施顺序

1. **批次 S0：稳定基线**
   - P0-1 `enva` 测试稳定化。
   - P0-2 凭据轮换。
   - P0-3 工作树和构建产物清理。

2. **批次 S1：阻止虚假成功与数据破坏**
   - P1-1 `gomats` staging/产物验证。
   - P1-2 `enva` transaction/lock/cache ownership。
   - P1-3 `enva` 可选兼容后端边界简化。

3. **批次 S2：冻结原生科学契约**
   - P1-4 `htseq2matrix-go` matrix schema。
   - P1-5 `qctb` report schema。
   - P1-6 `methrix-cli` custom HDF5 schema。

4. **批次 S3：异常输入与规模可靠性**
   - P2-1/P2-2 `methrix-cli` 流式处理和 genome provenance。
   - P2-3/P2-4 `fastqc-rs` 资源限制和外部格式验证。

5. **批次 S4：跨仓集成**
   - P3-1 RNA splicing。
   - P3-2 QC。
   - P3-3 甲基化。
   - P3-4 RNA count。

6. **批次 S5：发布交付**
   - 子仓提交、推送、主仓指针、干净环境复验和文档收口。

## 5. 统一验收命令

### Go 子仓

```bash
go test -count=1 ./...
go vet ./...
go test -race -count=1 ./...
CGO_ENABLED=0 go build ./...
```

### Rust 子仓

```bash
cargo fmt --check
cargo clippy --all-targets --all-features -- -D warnings
cargo test --all-features
```

### 主仓

```bash
go test -count=1 ./...
go vet ./...
git diff --check
```

所有命令必须在对应仓库或使用显式 `go -C` / `--manifest-path` 运行，避免错误地在主仓执行子仓门禁。

## 6. 明确不纳入发布阻塞的事项

- `qctb` 与历史 R QC 表结构一致。
- 生成 `qc_summary.RDS`。
- `qctb` 的 R/Rust golden diff。
- `methrix-cli` 的 `HDF5Array`、`SummarizedExperiment` 或 methrix R loader 兼容。
- 安装 R `rhdf5`、`HDF5Array` 或 `methrix` 作为发布前置条件。

这些事项后续如重新需要，应作为新的兼容层需求单独设计，不得隐式加入当前原生 schema。
