<p align="right">
  <a href="./README.md">English</a> · <strong>简体中文</strong>
</p>

<p align="center">
  <img src="./figs/logo.png" width="220" alt="OTTER Logo">
</p>

<h1 align="center">OTTER</h1>

<p align="center">
  <strong>Orchestrated Transcriptomic, Tumor-xenograft (PDX/CDX), and Epigenomic Reporting Workflow</strong><br>
  面向 RRBS、WGBS、RNA-seq 和 PDX 分析的可复现生物信息学工作流 CLI。
</p>

<p align="center">
  <a href="#它解决什么问题">核心功能</a> ·
  <a href="#快速上手">快速上手</a> ·
  <a href="#工作流体系">工作流体系</a> ·
  <a href="#组件">组件体系</a> ·
  <a href="#文档入口">文档中心</a>
</p>

---

Otter 将 FASTQ 和样本信息转换为经过校验的分析项目，并支持本地或 SLURM 执行、后台任务管理、断点恢复和可审计产物发布。

<p align="center">
  <img src="./figs/otter-workflow-stack.svg" width="100%" alt="Otter 从项目控制、Craftmake、Enva、领域算子到 Bamdriver 的工作流体系图">
</p>

## 它解决什么问题

- 通过 `init` 初始化项目，通过 `create` 扫描 FASTQ、校验配对并生成分析配置。
- 支持 RRBS、WGBS、RNA-seq、BS-PDX 和 RNA-PDX 场景。
- 支持本地执行与 SLURM 执行。backend 与每个 phase 的 CPU、内存、分区边界在解析 run 时固定，snapshot 在运行时具有权威性。
- 通过 `task list`、`task status`、`task logs`、`task stop` 和 `task report` 管理后台运行。
- 将规范项目文件解析为带 reference、资源和运行身份的不可变 `otter.run/v1` snapshot。
- 在运行边界验证 artifact manifest 和 checksum。

## 工作流体系

```text
otter → craftmake → enva → 算子 → bamdriver
  │         │         │         │          │
  │         │         │         │          └─ 共享 BAM/BGZF 基础能力
  │         │         │         └─ fastqcx · xenofilx · pairbam · seq2mat
  │         │         │            matsrun · qctb · methx
  │         │         └─ rattler-first 运行环境管理
  │         └─ 原生 Local/SLURM 执行与任务状态
  └─ 项目、配置、工作流和任务控制面
```

当前运行时明确采用双轨模型：已有生产流程使用 Snakemake 兼容路径；`craftmake` 是正在接入和验证的原生 Go 执行层。

## 快速上手

### 安装 release

发布安装器是发布到
[`otterlab-bio/otter`](https://github.com/otterlab-bio/otter) 的静态编译 Go 二进制：

```bash
curl -fsSL -o otter-install https://github.com/otterlab-bio/otter/releases/latest/download/otter-install-linux-amd64-static
chmod +x otter-install
./otter-install
```

旧的 shell 安装器仍保留用于兼容：

```bash
bash <(curl -fsSL https://raw.githubusercontent.com/otterlab-bio/otter/main/scripts/install.sh)
```

### 从源码构建

```bash
git clone --recurse-submodules https://github.com/otterlab-bio/otter.git
cd otter
conda activate go-env
go build -o otter .
```

### 创建并运行兼容路径项目

```bash
otter init my_project --legacy

otter create --legacy \
  --fastq /data/fastq \
  --mode RRBS \
  --pdata /data/samples.xlsx \
  --output my_project/userspace \
  --jobid demo_rrbs
```

legacy 的 `otter.yaml` 只是 authoring 格式，**两个执行层都不直接接受**。要运行它，先迁移到规范项目根目录、解析 run，然后执行该 snapshot：

```bash
otter init migrated
otter config migrate \
  --input my_project/userspace/demo_rrbs/config/otter.yaml \
  --output migrated/project.yaml \
  --reference-root /shared/otter/references \
  --reference-primary hg19@GRCh37.p13-gencode-v19

otter config resolve --project migrated/project.yaml \
  --reference-root /shared/otter/references --backend local

otter run --config migrated/runs/<run-id>/run.yaml \
  --executor snakemake --dry-run --foreground
```

`config migrate` 只写出项目意图，所以它需要一个已经带有 pinned workflow assets 的规范项目根目录。详见 [reference migration](docs/manual/08-reference-migration.md)。

backend 与每个 phase 的资源边界在解析时固定，运行时参数不能覆盖。默认运行会创建后台任务并打印 task ID：

```bash
otter task list
otter task status <task-id>
otter task logs <task-id> --follow
```

### 使用规范的不可变 run 模型

```bash
otter config validate --config project.yaml --schema v1
otter config resolve --project project.yaml --backend local
otter run \
  --config runs/<run-id>/run.yaml \
  --executor craftmake \
  --phase step1 \
  --backend local \
  --foreground
```

`craftmake` 要求不可变的 `otter.run/v1` snapshot。executor、backend、phase、reference 和资源边界必须在运行前解析；运行时参数不能静默覆盖 snapshot。

snapshot 把所有路径都记录为绝对路径，因此 `otter run` 不必在项目目录中启动：传绝对路径的 `--config`，在任何目录都能运行。唯一的例外是 `--run-id`，它是在当前工作目录下解析的；请配合 `--project-dir` 使用，或先 `cd` 进入项目。

`otter build` 把 init、create、validate 和 resolve 串成一步，在进入执行前停止，并打印 snapshot 路径。默认使用 craftmake 执行层；加 `--executor snakemake` 则在 snapshot 中记录 Snakemake 兼容执行层。

```bash
otter build \
  --project-root my_project \
  --fastq /data/fastq \
  --pdata /data/samples.csv \
  --mode RRBS \
  --reference-root /shared/otter/references \
  --reference-primary hg19@GRCh37.p13-gencode-v19 \
  --backend local
```

executor 属于 run snapshot，因此该选择不会写进 `project.yaml`，而且可以在不同 run 之间不同。已经存在 `project.yaml` 的目录会被拒绝而不是被覆盖：请改用 `otter config resolve` 从现有项目再解析一个 run。

## 组件

| 组件 | 角色 | 仓库 |
| --- | --- | --- |
| `otter` | 工作流项目和任务控制面 | [otterlab-bio/otter](https://github.com/otterlab-bio/otter) |
| `craftmake` | 原生工作流编译器和 Local/SLURM 执行层 | [otterlab-bio/craftmake](https://github.com/otterlab-bio/craftmake) |
| `enva` | Rattler-first 环境生命周期管理 | [otterlab-bio/enva](https://github.com/otterlab-bio/enva) |
| `fastqcx` | FASTQ QC 与 FastQC 兼容汇总输出 | [otterlab-bio/fastqcx](https://github.com/otterlab-bio/fastqcx) |
| `xenofilx` | PDX graft/host 读段分类 | [otterlab-bio/xenofilx](https://github.com/otterlab-bio/xenofilx) |
| `pairbam` | paired-end BAM 过滤与排序 | [otterlab-bio/pairbam](https://github.com/otterlab-bio/pairbam) |
| `seq2mat` | HTSeq count 转表达矩阵 | [otterlab-bio/seq2mat](https://github.com/otterlab-bio/seq2mat) |
| `matsrun` | rMATS 成对剪接分析编排 | [otterlab-bio/matsrun](https://github.com/otterlab-bio/matsrun) |
| `qctb` | 版本化 QC 汇总报告 | [otterlab-bio/qctb](https://github.com/otterlab-bio/qctb) |
| `methx` | Bismark coverage 和甲基化 HDF5 处理 | [otterlab-bio/methx](https://github.com/otterlab-bio/methx) |
| `bamdriver` | 共享纯 Go BAM/BGZF 库 | [otterlab-bio/bamdriver](https://github.com/otterlab-bio/bamdriver) |

历史源码符号和旧资产名称可能仍包含 `xdxtools`、`fastqc-rs`、`xenofilter-go`、`Paireads`、`htseq2matrix-go`、`gomats`、`methrix-cli` 或 `bamdriver-go`。这些名称只在兼容路径和历史证据中保留。

## 文档入口

- [文档中心](docs/README.md)：当前契约、教程和 schema 的入口。
- [用户手册](docs/manual/README.md)：安装、数据准备、快速上手、分析模式、高级用法、组件和 FAQ。
- [示例](docs/examples/)：完整的项目配置、reference、lock 文件和 run snapshot。
- [Schema](docs/schema/)：项目配置、run snapshot、reference、lock 和 artifact manifest 的 JSON Schema。
- [离线 e2e 演练](scripts/e2e/otter_e2e.sh)：在无网络环境下驱动真实二进制，验证 authoring、build、site、配对和迁移契约。
- [Methx → 原生 Methrix HDF5 转换函数](methx/scripts/export_methrix_hdf5.R)

## 开发

```bash
conda activate go-env
go test -v ./...
go vet ./...
```

Rust 子仓库使用 `rust_build` 环境。每个子仓库都是独立仓库，具体构建和测试命令以各自 README 为准。

## 许可证

MIT
