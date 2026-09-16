# enva 环境初始化指南

本指南详细介绍如何使用 enva 工具创建和管理 xdxtools 所需的 conda 环境。

## 前提条件

### 1. 安装 enva

```bash
# 从源码构建
cd enva
cargo build --release
cp target/release/enva ~/.cargo/bin/

# 验证安装
enva --version
```

### 2. 初始化 xdxtools-runtime

```bash
# 创建运行时目录
mkdir -p ~/xdxtools-runtime
xdxtools init ~/xdxtools-runtime
```

这会在 `~/xdxtools-runtime/` 下创建以下目录结构：
- `config/` - 配置目录
- `data/` - 基因数据库文件
- `envs/` - conda 环境 YAML 文件
- `R/` - R 脚本目录
- `rules/` - Snakemake 规则
- `snakefiles/` - 主工作流文件

## 环境创建

### 方式一：在 runtime 目录创建（推荐）

```bash
cd ~/xdxtools-runtime

# 创建所有环境
enva create --all

# 或单独创建
enva create --core      # xdxtools-core (包含 qualimap)
enva create --snakemake # xdxtools-snakemake
enva create --extra     # xdxtools-extra
```

### 方式二：使用自定义 YAML 文件

```bash
# 使用自定义配置文件
enva create --core --yaml /path/to/custom.yaml --name myenv

# 或在任意目录创建
enva create --yaml ./my-env.yaml --name myenv
```

## 环境说明

| 环境名 | 用途 | 主要工具 |
|--------|------|----------|
| xdxtools-core | 核心生物信息工具 | FastQC, MultiQC, Bismark, STAR, BWA, Bowtie2, Samtools, Qualimap 等 |
| xdxtools-snakemake | 工作流引擎 | Snakemake, Python 3.10+ |
| xdxtools-extra | 附加可视化工具 | 高级分析和可视化工具 |

**注意**: xdxtools-r 环境已废弃，R 脚本已由 Go 替代品替换：
- gomats → RNA_Splicing.R
- htseq2matrix-go → htseq2matrix.R

### xdxtools-core 包含的主要工具

- **质量控制**: FastQC, MultiQC, Qualimap
- **甲基化分析**: Bismark, Bowtie2
- **序列比对**: BWA, STAR, Bowtie2
- **变异检测**: Samtools, HTSlib, Picard
- **Peak calling**: MACS2, Homer
- **RNA-seq**: HISAT2, StringTie, HTSeq, rMATS
- **可视化**: matplotlib, seaborn

## 常用命令

### 查看环境列表
```bash
enva list
```

### 在特定环境中运行命令
```bash
# 运行单个命令
enva run xdxtools-core -- fastqc --version

# 运行 qualimap
enva run xdxtools-core -- qualimap --version

# 运行 Python
enva run xdxtools-snakemake -- python --version
```

### 验证环境
```bash
enva validate --all    # 验证所有环境
enva validate --core   # 验证特定环境
```

### 删除环境
```bash
enva remove --core     # 删除单个环境
enva remove --all      # 删除所有 xdxtools 环境
```

## 配置文件位置

enva 按以下顺序查找配置文件：

1. **当前目录的 `envs/` 文件夹**（xdxtools-runtime 布局）
   ```
   ~/xdxtools-runtime/envs/xdxtools-core.yaml
   ```

2. **当前目录的 `src/configs/` 文件夹**（开发模式）
   ```
   ./src/configs/xdxtools-core.yaml
   ```

3. **缓存目录 `~/.cache/xdxtools/configs/`**（自动复制）

## 故障排除

### 环境创建失败

```bash
# 查看详细错误信息
enva -v create --core

# 尝试 dry-run 模式
enva create --core --dry-run
```

### 包管理器优先级

enva 默认优先级：micromamba → mamba → conda

如需强制使用特定包管理器：
```bash
export ENVA_PM=micromamba  # 强制使用 micromamba
export ENVA_PM=conda        # 强制使用 conda
```

### 重新安装环境

```bash
# 删除现有环境
rm -rf ~/.local/share/mamba/envs/xdxtools-core

# 重新创建
cd ~/xdxtools-runtime
enva create --core
```

## 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| ENVA_PM | 强制使用指定包管理器 | auto (micromamba > mamba > conda) |
| MAMBA_ROOT_PREFIX | micromamba 根目录 | ~/.local/share/mamba |

## 相关文件

- `inst/envs/` - xdxtools 源码中的环境配置
- `enva/src/configs/` - enva 内置的环境配置
- `~/.cache/xdxtools/configs/` - 运行时缓存的配置
