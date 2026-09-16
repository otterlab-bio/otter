# 实施与进度审查报告（更新于 2026-03-05）

## 1. 审查目标
围绕既定定位审查：

- 主仓 `xdxtools` 负责 Snakemake runtime 管理、config 生成、流程进程与动态作业调度。
- 子仓负责工具调用抽象与复杂依赖隔离。

## 2. 本次已验证事实（2026-03-05）

1. 主仓核心执行链路已落地：`init/create/run/status`、engine 工厂、Snakemake executor、state 持久化与 resume。
2. 动态作业关键修复已在代码中：
- local 单样本并行命令包含 `--configfile`。
- state 步数按 workflow 动态初始化，不再固定 3 步。
3. 工具链一致性闭环已落地：
- `scripts/verify_toolchain_consistency.sh` 可通过。
- `scripts/install.sh`、`scripts/build-all-submodules.sh` 与 `inst/rules/*.smk` 所需工具集一致。
4. CI 门禁已覆盖 PR/Push；本次补齐 Release 流程一致性检查（`release.yml` 新增执行一致性脚本）。

## 3. 可执行证据

### 3.1 命令验证结果

| 命令 | 结果 |
|---|---|
| `conda run -n go-env go test ./internal/workflow ./internal/engine ./cmd -run "TestShouldPreflightRNAsplicing|TestLocalEngine|TestState|TestManager" -count=1` | 通过（3 个包均 `ok`） |
| `bash scripts/verify_toolchain_consistency.sh` | 通过（rules/install/build 三方一致） |
| `git submodule status` | 8 个子仓指针均可解析，无 detached/异常前缀证据 |

### 3.2 关键实现锚点

- 运行与 preflight：`cmd/run.go`
- 动态调度：`internal/workflow/manager.go`、`internal/engine/local.go`、`internal/engine/slurm_array.go`
- Snakemake 进程构建与 fallback：`internal/workflow/snakemake.go`
- 状态动态步数：`internal/workflow/state.go`
- 一致性门禁：`scripts/verify_toolchain_consistency.sh`

## 4. 进度评分（0-3）

| 维度 | 评分 | 说明 |
|---|---:|---|
| Runtime 管理 | 3 | 初始化、状态、resume、引擎切换完整 |
| Config 生成 | 3 | FASTQ 扫描/配对/pdata 校验/配置输出完整 |
| 进程管理 | 3 | Snakemake 命令构建、环境校验、fallback 完整 |
| 动态作业 | 2 | local/slurm 路径实现完整，缺端到端运行证据 |
| 子仓解耦 | 3 | 依赖工具映射与构建/安装一致性已闭环 |

**加权完成度：约 90/100。**

## 5. 当前剩余风险（按优先级）

### P1
1. 发布前最小运行回归尚未形成固定记录：
- RRBS `run --dry-run`（local）
- RNASEQ `run --dry-run`（slurm）

### P2
1. 审查文档需要在每次 release 前自动更新时间戳与命令输出摘要，避免“文档过期但脚本已更新”的漂移。

## 6. 结论
当前项目已符合既定职责划分，且“rules-安装-构建-CI”一致性闭环可用。发布阻塞从“实现缺失”转为“回归证据沉淀”，建议将最小 dry-run 回归纳入发布门禁后再做常态化发版。
