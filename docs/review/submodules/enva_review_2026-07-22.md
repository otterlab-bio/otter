# enva 系统审查报告（2026-07-22）

## 1. 审查结论

- **初始仓库基线**：本地 HEAD `b7f45e6336dfa33e9fbeaab89f2710a3a437524c`（`master`，相对 `origin/master` ahead 5）
- **初始工作树**：5 个源文件未暂存改动（`src/backend/mod.rs`、`src/backend/rattler.rs`、`src/lib.rs`、`src/main.rs`、`src/micromamba.rs`），属用户原有改动；后续整改在其上叠加并保留
- **初始审查状态**：只读审查发现 1 Critical / 7 High / 5 Medium / 2 Low
- **当前发布结论**：本报告内 rattler 主链的 Critical/High 与既定 capability/remove ownership 阻塞已收口；micromamba 自动安装能力已从正式路径删除。剩余为兼容后端真实 smoke test、native install 边缘覆盖和仓库级集成，Windows 按范围决策延期。

本地未推送的 5 个提交为 shell hook、交互式多环境删除、toolchain 配置等；脏树改动（`bash -lc`→`bash -c`、补 `--`、禁 banner、banner 移 stderr）改善了主仓机器调用的输出污染和部分参数分隔，但**未修复 argv 边界丢失、shell 注入或 Windows 不可用**。

## 1.1 整改后状态（2026-07-24）

- Critical 路径逃逸和递归删除边界已通过强类型 `EnvironmentName`、canonical containment、symlink 拒绝及 root/home/workspace 保护收口。
- argv 与显式 shell 模式已分离，主仓标准 argv 调用不再被重拼进 shell。
- `create --force` 使用 sibling staging、产物验证、旧目录 backup、rename publish 和版本化 JSONL journal；journal 采用 append + `sync_all()` 持久化，允许在尾记录截断时回退到最后一个完整状态。
- create 取得 prefix lock 后会先恢复遗留事务；恢复只操作与 final parent、transaction ID 和 allocation suffix 精确匹配的 sibling staging/backup，损坏或矛盾状态 fail closed。
- 子进程在 `final → backup` 后直接退出的测试证明下一次恢复可找回旧环境；故障注入覆盖 parent sync 和 backup cleanup，恢复可重复执行。
- create/install/remove 使用 prefix lock；solve/install/cache cleanup 使用同一 cache-root advisory lock；父/子测试进程证明锁跨进程生效。
- ownership marker 原子写入，损坏 marker 和无法安全 adopt 时 fail closed；cache cleanup 仅清理由 marker 声明的安全根目录。
- rattler 是正式默认后端；micromamba、mamba、conda 仅作为显式兼容工具。
- micromamba 自动下载、解包、固定资产、hash 和原子安装路径已退出正式实现；只接受 `ENVA_MICROMAMBA_PATH` 或 `PATH` 中已有且通过 `--version` 健康检查的可执行文件。
- 显式 `ENVA_MICROMAMBA_PATH` 无效时直接失败，不回退到 `PATH`；直接构造或通过 `--pm` 显式请求不可用兼容后端时不再静默切换。
- 已删除直接 `reqwest`/`sha2` 依赖；`Cargo.lock` 继续作为 CLI 锁文件跟踪。
- 最新门禁通过：`cargo fmt --check`、严格 Clippy、默认测试（120 deterministic library passed + 4 network integration ignored + 4 CLI passed）、4 个网络集成测试单独通过、`cargo check --locked`、主仓 `go test -count=1 ./...` / `go vet ./...` 和 `git diff --check`。
- 本轮新增：事务恢复统一接入 name/prefix discovery、list、run、install、remove 和 adopt；run 不再隐式 adoption，install 外部环境要求先显式 `enva adopt`；MatchSpec 参数保留逗号边界；非零子进程退出统一保留为结构化 `ProcessExit`，包括 141；YAML dry-run 在 rattler 后端执行真实 repodata solve，并把 `--with` specs 纳入求解；native install 克隆到 sibling staging，在 staging 执行 Installer、恢复 ownership 并验证后通过 journal 原子发布。
- 真实生命周期门禁覆盖 create、solver 失败、Installer 完成后注入失败、成功安装 Python、最终 `sys.prefix`/`python3.10-config --prefix` 校验及 remove；两类失败均保留旧 prefix、sentinel 和 ownership marker，并清理 staging/backup/journal。

- 真实 cache 并发门禁覆盖真实 conda-forge solve 与独立 cleanup 子进程：solve 持有 `CacheUse` 时 cleanup 阻塞，释放后才删除 `pkgs`、`repodata`、`run-exports`，并保留 cache ownership marker。

- `CONDA_PREFIX` root 推导改为类型化 fail-closed 分类：标准 `<root>/envs/<name>` 推导 parent root，显式 `CONDA_DEFAULT_ENV=base` 接受 prefix 本身，外部、自定义和相对 prefix 不生成 root candidate。

- canonical environment identity 与同名解析已收口：存在 prefix 用 canonical path 去重；名称匹配按 `NotFound` / `Unique` / `Ambiguous` 类型化；run/activate/install/adopt/remove 仅在唯一时执行，不同物理 prefix 同名时要求显式 `--prefix`；歧义 install/remove 不写 ownership 且不删除任何 prefix。

- typed capability 与 remove ownership 已收口：`BackendCapabilities` 以 `Native` / `Delegated` / `Hybrid` / `Unsupported` 描述 create/validate/install/adopt/remove/discovery/run/cache 能力，命令入口在具体操作前 fail closed；rattler remove 只接受 rattler-owned 环境，external 环境必须先独立执行 `enva adopt`，拒绝路径不写 ownership marker、不删除 prefix。

本报告内既定 rattler 主链代码阻塞已关闭。Windows 兼容按当前范围决策延期；兼容后端真实 smoke test 和 native install 的 hard-link/relocation/大 prefix 性能覆盖继续作为后续互操作与健壮性任务。

## 2. 门禁结果

| 命令 | 结果 |
|---|---|
| `cargo fmt --check` | 通过 |
| `cargo clippy --all-targets --all-features -- -D warnings` | 通过（整改后复验） |
| `cargo test --all-features` | 通过（120 deterministic library passed、4 network integration ignored、4 CLI passed） |
| `cargo test --all-features -- --ignored` | 通过（2 个真实 conda-forge solve 场景 + 1 个真实 create/install rollback/publish/remove 场景 + 1 个真实 solve/cache cleanup 跨进程并发场景） |

环境中无 `conda`/`mamba`/`micromamba`，使用系统 Rust 1.97.1，因此本轮未执行真实兼容后端互操作；该验证不阻塞 rattler 主链。

初始审查时的 Clippy 问题包括 test module 后公开 item、unit struct `default()`、可派生 `Default`、测试中 `MutexGuard` 跨 `.await`、`print_literal` 和 `needless_borrows_for_generic_args`；整改后已全部通过 `-D warnings`。

审查过程中 cargo 门禁生成的临时构建目录（`target-native-tests-20260722`、`target-paracloud-musl-20260722`）已清理，用户 5 个源文件改动保持原样。

## 3. Critical 发现

### ENVA-C01：环境名可逃逸允许根目录，并被 `create --force` 递归删除

- **位置**：`src/backend/rattler.rs:62-110,128-136,1410-1500`。
- **触发**：`enva create --yaml env.yaml --name /absolute/path/to/victim --force` 或 `--name ../../victim --force`。
- **证据**：`target_prefix_for_env_name()` 只拒绝 `"base"`；直接 `preferred_root_prefix().join("envs").join(env_name)`；Rust 路径连接遇绝对路径丢弃前根，`..` 不规范化/拒绝；目标含 `conda-meta/` 时 `--force` 执行 `remove_dir_all()`；无 canonicalize/symlink 策略/必须位于允许根目录内的检查。
- **影响**：可在 rattler root 外创建环境；对任意 conda-style 目录递归删除；root 为符号链接时也可能越界。
- **整改**：强类型 `EnvironmentName` 只允许一个正常路径组件；拒绝绝对/`.`/`..`/分隔符/Windows drive/NUL/空；canonicalize 父目录后验证位于 allowed root `envs/` 下；删除前 `symlink_metadata` 拒绝符号链接；`remove_dir_all` 前验证所有权 marker + 允许根目录 + 非 base + identity。

## 4. High 发现

### ENVA-H01：`run -- <argv>` 丢失参数边界进入 shell，破坏主仓契约且允许注入

- **位置**：`src/env_run.rs:71-93,420-438,461-474`、`src/backend/mod.rs:13-22`、`src/backend/rattler.rs:1235-1270`、`src/micromamba.rs:1300-1375`、主仓 `internal/enva/enva.go:16-38`、`internal/engine/local.go:286-301`。
- **证据**：主仓用独立 argv 调 `enva run env -- tool --output "a b" "$(payload)"`；enva 将 argv 重拼成空格分隔字符串交 `bash -c`；`--script` 也 `format!("Rscript {}", script.display())` 未引用。
- **影响**：`"a b"` 变两参数；`*`/`$()`/反引号/`;`/重定向/管道被 shell 解释；主仓 Go 侧 `exec.Command` 的安全 argv 保护在 enva 内被破坏。
- **整改**：建模 `Argv(Vec<OsString>)`（默认不经 shell）与 `ShellCommand(String)`（仅 `--command`）；`--script` 用独立 argv；主仓契约测试覆盖空格/引号/`$()`/通配/换行/前导 `-`。

### ENVA-H02：`create --force` 先删旧环境再求解安装，无事务/回滚

- **位置**：`src/backend/rattler.rs:1380-1535`、`src/micromamba.rs:1154-1288`。
- **影响**：网络/solver/磁盘错误或中断后原环境永久丢失；无恢复点。
- **整改**：已实现临时 prefix 完成 solve/install/验证/marker，成功后原子 rename（旧→备份，新→正式），最终删除备份；版本化 journal 记录发布阶段并在下一次锁内操作前恢复中断状态。

### ENVA-H03：锁只在单 backend 实例内生效，无法防跨进程/跨操作竞争

- **位置**：`src/backend/rattler.rs:35-59,1360-1370`、`src/backend/factory.rs:6-17`、`src/micromamba.rs:170-191,1154-1165`。
- **证据**：`creation_lock` 为实例内 `Arc<Mutex<()>>`，每次 `build_backend()` 新建；不跨进程；install/remove/cache cleanup 不共享 prefix 锁。
- **整改**：按 canonical prefix 跨进程 lock file；cache root 共享/排他锁；锁含 PID/时间/操作/transaction ID，处理 stale。

### ENVA-H04：普通 `run`/`install` 隐式 adopt 外部环境，所有权边界失效

- **位置**：`src/backend/rattler.rs` 的 run/install/adopt 入口。
- **证据**：旧实现中 `run()`/`install_packages()` 调 `ensure_adopted_environment()` 直接写 rattler marker；这会在用户未明确授权时改变外部环境元数据。
- **整改**：`run`、install 和 remove 都只接受 rattler-owned environment，不再写 marker 后继续操作 external prefix；external 环境必须先独立执行显式 `enva adopt --name/--prefix`，或使用正确的兼容包管理器直接操作。remove 的 name/prefix 拒绝测试证明目录与 ownership marker 均保持不变；adopt 自身仍在 prefix lock 内恢复事务并原子写 marker。

### ENVA-H05：cache cleanup 对任意环境变量目录递归删除，后端语义不一致

- **位置**：`src/backend/rattler.rs:983-1037,1295-1345`、`src/env.rs:454-456`、`src/micromamba.rs:1111-1148`。
- **证据**：`RATTLER_CACHE_DIR` 无条件接受为 cache root，直接递归删 `pkgs/`/`repodata/`/`run-exports/`；无 allowlist/marker/symlink 验证；CLI 后端执行 `conda|mamba|micromamba clean --all` 影响全局。
- **整改**：只清理由 enva 创建且有 marker 的 cache；拒绝 `/`/home/工作区/当前目录及祖先；输出 canonical 列表要求确认；拆分 rattler cache cleanup 与全局 clean。

### ENVA-H06：自动下载 micromamba 缺版本固定/校验和/原子写入/安全解包

- **位置**：`src/micromamba.rs:648-665,686-870,879-948`。
- **证据**：可变 `latest` URL；只依赖 HTTPS 无 SHA-256/签名；直接写最终路径；外部 `tar -xjf` 解压无 entry 路径校验；存在即跳过。正面：HTTP 客户端用 rustls TLS，未发现禁用证书验证。
- **整改**：删除自动下载与安装能力；micromamba 仅从 `ENVA_MICROMAMBA_PATH` 或 `PATH` 发现，并执行普通文件、执行权限和 `--version` 健康检查；显式路径或后端不可用时 fail closed。

### ENVA-H07：已修复。同名环境统一 fail closed，prefix identity canonicalize

- **原位置**：`src/backend/rattler.rs`、`src/prefix_registry.rs`、`src/env_run.rs`、`src/env.rs`。
- **原证据**：去重 key 为原始 `PathBuf`；run/activate 按优先级选首个仅 warning；install/adopt 要求唯一；remove 采用另一套交互规则。
- **整改**：新增 `EnvironmentResolution<T> { NotFound, Unique, Ambiguous }`；已存在 prefix 使用 canonical path 作为物理身份去重键，symlink alias 折叠为同一环境，真正不同的同名 prefix 保持多个候选。run、activate、install、adopt 和 remove 的名称路径全部要求唯一，多匹配返回包含全部 prefix 的错误并要求 `--prefix`。install/remove 新增显式 prefix CLI 路径；回归测试证明歧义 install 不写 ownership，歧义 remove 不删除或 adopt 任一 prefix。

## 5. Medium 发现

- **ENVA-M01**：ownership marker 非原子写入，解析失败被静默降级为 external（`.ok().flatten()` 丢弃错误）。
- **ENVA-M02**：兼容后端的 YAML validation 仍只做本地结构检查；rattler 默认后端已改为真实 repodata solve，求解失败会使 dry-run 失败。
- **ENVA-M03**：安装参数原先按裸逗号拆分破坏合法 MatchSpec；已改为每个 CLI 参数保持完整 spec，并加入 `numpy>=1.24,<2` 回归测试。
- **ENVA-M04**：运行路径硬编码 `bash`，与声明 PowerShell/Windows 支持矛盾（脏树集中到 `backend/mod.rs`）。
- **ENVA-M05**：已修复。`CONDA_PREFIX` 仅在标准 `<root>/envs/<name>` 布局或 `CONDA_DEFAULT_ENV=base` 时贡献 rattler root；外部、自定义和相对 prefix fail closed，不再被当作 root。

## 6. Low 发现

- **ENVA-L01**：已修复。退出码 141 不再按错误字符串转换为成功；rattler 与兼容 backend 均返回结构化 `ProcessExit { code }`，回归测试确认 141 向上传播。
- **ENVA-L02**：全局 `--quiet` 只控制 banner 不控制命令输出（`main.rs:15-20,38-57`）。

## 7. 主仓调用契约检查

主仓多处按标准 argv 调 `enva run <env> -- <command> <arg1> ...`（`internal/enva/enva.go:16-38,55-67`、`internal/engine/local.go:286-301`、`cmd/run.go:619-652`），预期 argv 保持、无 shell 二次解释、stdout 不被 banner 污染、退出状态准确。

整改后：argv 模式保持原始参数边界且不进入 shell；显式 `--command` 才使用 shell；banner/输出模式已覆盖 CLI 测试；退出码 141 与其他非零退出一样准确向上传播；同名环境在 canonical identity 去重后仍多匹配时 fail closed，必须使用显式 prefix。仍待处理的是 SLURM 字符串模板的更强静态/集成门禁。

## 8. 完成门禁缺口

- [x] 环境名单一路径组件强类型验证。
- [x] destructive prefix canonicalize 并验证位于允许根目录内。
- [x] symlink 策略明确并测试。
- [x] base/root/home/workspace/当前目录永不可递归删除。
- [x] `run -- argv` 保留真实 argv 不经 shell。
- [x] `--command` 与 argv 模式类型/CLI 分离。
- [x] 主仓契约测试覆盖特殊字符/空格。
- [x] `run` 不隐式 adopt；外部环境 install 要求显式 adopt；ownership 损坏时 fail closed。
- [x] 外部环境 remove 的 ownership 语义统一：默认 rattler 后端不隐式 adopt，要求先显式 `enva adopt`；name/prefix 拒绝路径不写 marker、不删除 prefix。
- [x] ownership marker 原子写入、损坏 fail closed。
- [x] prefix 和 cache 跨进程锁。
- [x] force replacement 用 staging/原子切换/rollback。
- [x] 中断 journal/recovery。
- [x] cache cleanup 有 marker/allowlist/预览。
- [x] micromamba 自动安装能力从正式路径删除；兼容模式仅发现显式路径或 `PATH` 中已有二进制。
- [x] rattler 默认后端的 YAML dry-run 执行真实 solve；typed capability matrix 明确 rattler 与 CLI compatibility 的 Native/Delegated/Hybrid/Unsupported 语义，unsupported 操作在命令边界失败。
- [x] `CONDA_PREFIX` root 推导只接受标准 `envs/<name>` 或显式 base，外部/自定义/相对 prefix fail closed。
- [x] 同一物理 prefix 以 canonical path 去重；不同物理 prefix 同名时 run/activate/install/adopt/remove fail closed 并要求显式 `--prefix`。
- [x] MatchSpec 不按裸逗号拆分。
- [ ] CI 覆盖 Linux/macOS/Windows。
- [x] `cargo clippy --all-targets --all-features -- -D warnings` 通过。
- [x] 安全负向测试在隔离临时目录通过。
