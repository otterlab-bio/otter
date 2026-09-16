# Otter benchmark plan

> **Current status:** The accepted Gate 6 scope includes bounded real-SLURM Craftmake–Snakemake executor evidence, corrected Gate A–D evidence, and Methx/Methrix parity. The fresh seven-input legacy-equivalent matrix, representative `20 samples × 3 repeats`, production-scale scheduler-pressure qualification, WGBS requalification, and additional Snakemake recovery are deferred extensions.

> **Paracloud access update (2026-09-06):** SSH was restored in pseudo-terminal mode. Craftmake generated and validated 8 paired FASTQ fixture sets and 9 BAM fixture sets in isolated `build-work` directories. The 9 BAM fixtures follow the aligned-BAM-per-SRA policy (single reference for RRBS/RNA-seq; both hg38 and mm10 for PDX projects), each with 20,000 complete paired templates; filtered (Xenofilx) outputs and the retired `SRR23802966` line are excluded. Every generated file is below 20 MiB; paired counts, BAM sorting/indexing, checksums, and manifests passed. WGBS `SRR6373947` remains unavailable and clean-server acceptance is still pending.


> **Evidence boundary:** The corrected Gate A–D accounting is accepted within its documented scope. A complete artifact manifest proves publication integrity; it does not prove scientific parity by itself.


本计划将“已有对比数据”“可用于技术判断的对比数据”和“可用于生产放行的对比数据”严格区分。所有时间均按实际 Paracloud Slurm 队列可用性估算，不把排队时间伪装成执行时间。

| 阶段 | 工作内容 | 交付物 | 预计耗时 | 放行条件 |
|---|---|---|---:|---|
| P0–P1 | 运行时/参考前置条件与 RRBS recovery closeout | Completed | 已有 immutable runtime/reference、RRBS recovery/semantic evidence，以及 corrected Gate A–D evidence |
| P2 | RNA-seq、BS-PDX、RNA-PDX paired canary | Completed, bounded | 三场景均已完成 fresh paired executor evidence；不等同于生产数据或代表性 acceptance |
| P3 | PDX paired scheduler evidence 与 Methx/Methrix parity | Completed, bounded | PDX `step2-check` 三次 paired evidence、production Methx/Methrix matrix parity 与 HDF5 interoperability 已登记 |
| P4–P5 | Fresh seven-input matrix、代表性 repeats、scale/WGBS/Snakemake recovery | Deferred, non-blocking | fresh matrix 未执行；其余项目作为未来 qualification，不属于当前 Gate 6 release criteria |
| P10 | Evidence register、decision record 与条件性 publication verification | Current closeout | 仅在正式发布要求存在时执行 BS-PDX publication/artifact verification |

**预计获得数据的时间点：**

- **第一批可用于技术判断的对比数据：** P1 完成后，约 1–3 个工作日；范围是 RRBS，包含现有 r16b artifact parity、r17b reconciliation 和新增 recovery/semantic 结果。
- **四个 active non-WGBS 场景的完整 canary 对比数据：** P2 完成后，约 4–8 个工作日；包括 RRBS、RNA-seq、BS-PDX、RNA-PDX 的 Craftmake vs Snakemake paired reports。
- **可用于代表性生产决策的统计数据：** P4 完成后，约 7–15 个工作日；这是 20-cell × 3-repeat 矩阵之后的中位数、范围和 incident 汇总，不是单次 canary 的性能结论。

若出现 queue timeout、resource exhaustion、tool invocation 或 artifact integrity incident，估算时间顺延；不得通过跳过 recovery 或 semantic review 来提前宣称通过。WGBS 在本计划中仍 deferred。

## 1. Benchmark Matrix

每场景 × toolchain 组合的测试矩阵：

| Scenario | Modern | Legacy-equivalent | Canary (1-sample) | Repr (20-cell × 3 runs) | Scale |
|---|---|---|---|---|---|
| RRBS | ✅ fastqcx + methx + qctb | FastQC + Bismark + Methrix | ✅ bounded executor parity, recovery suite, semantic review | □ | □ |
| WGBS | ✅ fastqcx + methx + qctb | FastQC + Bismark + Methrix | □ deferred | □ | □ |
| RNA-seq | ✅ fastqcx + seq2mat + matsrun + qctb | FastQC + STAR + HTSeq + rMATS | ✅ bounded executor parity through publication | □ | □ |
| BS-PDX | ✅ fastqcx + xenofilx + methx + qctb | FastQC + XenofilteR + Bismark + Methrix | ✅ bounded executor parity; 3-repeat `step2-check` scheduler evidence | □ | □ |
| RNA-PDX | ✅ fastqcx + xenofilx + seq2mat + matsrun + qctb | FastQC + XenofilteR + STAR + HTSeq + rMATS | ✅ bounded executor parity; 3-repeat `step2-check` scheduler evidence | □ | □ |

**Current status:** The matrix below is a planning and evidence index. A check mark means that the toolchain mapping or bounded test exists; it does not mean production-scale qualification is complete.

## 1.1 Seven-Decoded-FASTQ Toolchain Comparison (2026-08-15; deferred extension)

The seven Paracloud-ready paired FASTQ datasets listed below were prepared as a potential fresh legacy-equivalent comparison corpus. The fresh matrix was not run and is no longer a Gate 6 release criterion. The corpus and its provenance remain documented for future authorized work; this section must not be read as completed seven-input scientific evidence.

Every comparison run must be created by `otter config validate` and `otter config resolve`, which materialize a fresh immutable `otter.run/v1` snapshot. Otter owns project, sample, reference, acquisition-provenance, and input-identity validation. Craftmake is the only workflow control plane: `otter run --executor craftmake` delegates planning, submission, resume, status, logs, and reporting to Craftmake. Production workflow tasks must not be submitted through handwritten `run.yaml` files or direct `sbatch` invocations.

All seven inputs follow the same fixed pre-processing boundary: raw paired FASTQ receives modern `fastqcx` and legacy FastQC + SeqKit QC, then the same pinned Trim Galore invocation, then modern and legacy QC of the trimmed pair. Trim Galore is a shared fixed transformation in this comparison; it is not a new-versus-legacy trimming axis. The downstream cells are RRBS (`fastqcx + methx + qctb` versus `FastQC + Bismark + Methrix`), RNA-seq (`fastqcx + seq2mat + matsrun + qctb` versus `FastQC + STAR + HTSeq + rMATS`), BS-PDX (`fastqcx + xenofilx + methx + qctb` versus `FastQC + XenofilteR + Bismark + Methrix`), and RNA-PDX (`fastqcx + xenofilx + seq2mat + matsrun + qctb` versus `FastQC + XenofilteR + STAR + HTSeq + rMATS`). `pairbam` and `bamdriver` are compared only in BS (RRBS/WGBS) and BS-PDX paired-BAM phases; they are not RNA-seq or RNA-PDX operators.

Snakemake remains an explicit compatibility/rollback executor only. The planned seven-sample toolchain matrix is Craftmake-driven; only the separately documented real-workflow interruption/retry, publication/recovery, and compatibility-smoke debt may use Snakemake.

## 1.2 BAM/NM and PDX classification prerequisite — accepted (2026-09-05)

The corrected Gate A–D prerequisite is complete for the accepted Gate 6 scope. `bamdriver` preservation, `pairbam` fragment/mate preservation, Xenofilx NM validation against an independent oracle, CT/GA controls, and fragment-level Xenofilx versus `Picard NM patch + XenofilteR` accounting are recorded in the [closeout evidence register](gate6-closeout-evidence-register.json). Gate D covers all `1,115,239` modern-only disagreements with zero missing score records and zero unexplained disagreements; exact modern/legacy membership equality is not claimed.

## 2. Parity Criteria

| Tier | 要求 | 通过条件 | 不通过处理 |
|---|---|---|---|
| **Exact** | 规范化输出完全一致 | byte-for-byte or sorted-text identical | blocker |
| **Structural** | 文件格式、字段、维度、样本顺序、reference identity 一致 | 字段级 diff = 0 | blocker，允许格式转换豁免 |
| **Scientific** | 领域容差内结果等价 | 预定义 tolerance（甲基化 ±0.01、counts ±5%、splicing ±1% PSI） | 需 root-cause 分析 + waiver |
| **Informational** | logs/images/timestamps 合理 | 无硬性要求 | 记录偏差，不阻断 |

## 3. Cluster dependencies

The accepted Gate 6 scope does not include a new representative matrix, production-scale pressure test, WGBS requalification, or additional Snakemake recovery exercise. Existing real-SLURM paired evidence remains bounded acceptance evidence; future qualification work requires a new approved matrix and evidence register entry.

| Gate 6 子项 | 集群需求 | 状态 |
|---|---|---|
| canary | Accepted Paracloud runtime/reference plus checksum-verified fixtures | RRBS, RNA-seq, BS-PDX, and RNA-PDX bounded executor parity accepted |
| representative | Slurm + approved 20-cell inputs + ≥3 runs per cell | Deferred; no 60-run matrix submitted |
| failure injection | Local backend plus real-Slurm recovery evidence | RRBS recovery and publication retry accepted; additional scenario-specific recovery deferred |
| scale | Production-scale data + scheduler-pressure monitoring | Deferred future qualification |
| metrics | Versioned metrics/evidence schema and report generation | ✅; PDX repeated scheduler source/summary/chart retained |

## 4. Gate 6 Reference-build Baseline

Gate 6 reference releases are built on Paracloud compute nodes, not login nodes. `mm10` and `mm38` are aliases for the same `GRCm38` assembly and therefore share one immutable release rather than creating divergent duplicate indexes.

| Logical ID | Assembly / annotation | Source release | Gate 6 aliases | Required assets |
|---|---|---|---|---|
| `hg19` | GRCh37.p13 / GENCODE v19 | GENCODE human release 19 | `human`, `grch37` | FASTA, FAI, GTF, Bismark/Bowtie2/STAR indexes |
| `hg38` | GRCh38 / GENCODE v44 | GENCODE human release 44 | `human`, `grch38` | FASTA, FAI, GTF, Bismark/Bowtie2/STAR indexes |
| `mm10` / `mm38` | GRCm38 / GENCODE M25 | GENCODE mouse release M25 | `mouse`, `mm10`, `mm38`, `grcm38` | FASTA, FAI, GTF, Bismark/Bowtie2/STAR indexes |
| `mm9` | NCBIM37 / GENCODE M1 | GENCODE mouse release M1 | `mouse`, `mm9`, `ncbi37` | FASTA, FAI, GTF, Bismark/Bowtie2/STAR indexes |

The four production releases are accepted: provider archives were downloaded to controlled caches with HTTP Range resume, provider checksums were verified, canonical FASTA/GTF assets and SHA-256 evidence were generated, and indexes were built with the accepted `otter-core` runtime before atomic publication. Each release retained Craftmake state, controller logs, Slurm accounting, source manifests, and final manifest digest. Their compute-node integrity, readonly sealing, visibility, and `mm38` alias evidence are recorded in `docs/gate6-paracloud-operations.md`.

The first bounded build is `mm10-canary@GRCm38-ensembl-100-chr19`: Ensembl release 100 GRCm38 chromosome `19` FASTA with the matching Ensembl release 100 GTF filtered to contig `19`, plus Bismark/Bowtie2/STAR indexes. Both source archives publish BSD `sum` provider checksums and use the same unprefixed contig namespace. This is deliberately a technical canary for Craftmake/Slurm/index/publication/recovery validation; it must never be reported as a full `mm10`/`mm38` result or used for scientific parity. A successful canary de-risks the larger `mm10`/`mm38` release but does not satisfy its acceptance criteria.

## 5. Benchmark Metrics Schema

```json
{
  "benchmark_id": "bs-repr-20260726T000000Z",
  "run_ids": ["run-20260725T120000Z-abc123", "run-20260725T140000Z-def456"],
  "scenario": "rrbs",
  "toolchain": "modern",
  "backend": "slurm",
  "metrics": {
    "wall_clock": {"min_ms": 0, "max_ms": 0, "avg_ms": 0},
    "cpu_seconds": {"min": 0, "max": 0, "avg": 0},
    "peak_memory_mb": {"min": 0, "max": 0, "avg": 0},
    "io_read_mb": {"min": 0, "max": 0, "avg": 0},
    "io_write_mb": {"min": 0, "max": 0, "avg": 0}
  },
  "parity": {
    "tier": "scientific",
    "status": "pending",
    "comparator": "methrix-semantics/v1",
    "modern_artifacts": ["results/methylation/matrix.h5"],
    "legacy_artifacts": ["results/methylation/legacy_matrix.h5"],
    "tolerance": {"methylation_beta": 0.01, "coverage": 0.05}
  },
  "evidence": {
    "config_digest": "sha256:...",
    "reference_digest": "sha256:...",
    "workflow_digest": "sha256:...",
    "tool_digests": {"fastqcx": "sha256:...", "methx": "sha256:..."}
  }
}
```

## 6. Failure Injection Tests (local backend)

| 测试 | 场景 | 预期行为 | 状态 |
|---|---|---|---|
| `cancel` | 运行中发送 SIGTERM → exit code 8 | craftmake 收到 cancel → cancelled state → state.sqlite 标记 cancelled | ✅ 已实现（`exit_codes_integration_test.go`） |
| `task failure` | 步骤故意失败（exit ≠ 0） | task 标记 failed → controller 记录 attempt.failed → exit code 5 | ✅ 已实现 |
| `digest drift` | snapshot digest 与 runtime 不一致 | 检测到 drift → fail before sbatch | ✅ 已实现（`run_test.go`） |
| `controller loss` | 进程崩溃后 resume | orphan task 检测 → 重新调度 → 复用 cache | ✅ local contract；RRBS r18 real-Slurm controller interruption/resume、publication retry、failed-task retry passed |
| `reference digest mismatch` | reference.yaml 声明 digest ≠ 实际文件 | `VerifyChecksums` 失败 → fail before sbatch | ✅ 已实现（`manifest.go`） |

## 7. Run Metrics Collection

每个 run 写入 `run.yaml` 后，Craftmake 执行过程中产生：

- `<run_root>/metrics/run.json` —  wall clock、CPU、memory、I/O
- `<run_root>/metrics/tasks.json` — per-task 指标
- state.sqlite 中记录的 controller events（log-structured）

benchmark 聚合工具读取 N 个 run 的 metrics 并生成 `benchmark.json`。

## 8. Production Validation Contract

The authoritative four-scenario input/executor/toolchain matrix is [Gate 6 Canary Matrix](gate6-canary-matrix.md). Its dimensions are intentionally independent: executor parity fixes toolchain and compares Craftmake-default to explicit Snakemake compatibility; toolchain parity fixes executor and compares modern to legacy-equivalent implementations. Rust Bismark 3.1.0 and Bowtie2 2.5.5 are pinned shared dependencies in both paths. Perl Bismark is not an input, oracle, performance baseline, or result-comparison axis.

### Required Metrics and Interpretation

| Group | Required fields | Gate interpretation |
|---|---|---|
| reproducibility | input/reference/workflow/tool/run digest, executor/backend/site, selection seed/rule, resource request/source, task/attempt/job IDs, cache decision, artifact manifest digest | missing or mismatched identity is a blocker |
| correctness | artifact verify result, exact/structural report, methylation beta/coverage, RNA feature/count agreement, PDX classifications/mapped reads, splicing outcome/PSI | exact and structural failures block; scientific differences require root cause and approved expiry-bound waiver |
| reliability | completion rate, typed incident category, retry count, queue delay, scheduler state/exit/signal, cancellation latency, recovery result, publication idempotency | any unclassified or unresolved blocker incident blocks representative work |
| resource efficiency | wall time, CPU seconds, allocated/requested CPU, peak RSS/requested memory, read/write I/O, artifact bytes, task critical path, queue delay, metric quality | report repeated-run median and range; do not promote based on a single run |
| scientific QC | raw/trimmed reads and retention, alignment/mapping rate, duplicates when emitted, bisulfite conversion/coverage, RNA assignment, PDX category proportions | deviations require predeclared scenario baseline or an incident/waiver |

Craftmake `report` now writes `task_metrics.csv`, `step_timings.csv`, `allocations.csv`, and `run-evidence.json`. The evidence bundle binds run identity and task statuses to retained controller logs, accounting exports, and the versioned runtime incident records.

### Runtime Incident Policy

Failed or cancelled attempts produce `otter.runtime-incident/v1` records. Categories are `input_reference_digest`, `environment_tool`, `scheduler_submission`, `queue_timeout`, `resource_exhaustion`, `filesystem_io`, `network_acquisition`, `tool_invocation`, `scientific_qc`, `artifact_integrity`, `cancellation_recovery`, and `internal_unknown`. Each record includes retry safety/policy, owner, escalation rule, remediation state, backend job identity, exit/signal, and bounded diagnostic/evidence paths. Free-form stderr remains human evidence rather than a stable classifier.

Automatic retry is permitted only for bounded scheduler-submission or network-acquisition incidents. Cancellation/recovery and tool invocation require reviewed manual retry. Resource exhaustion requires a new immutable run with reviewed resources. Input/reference, environment, filesystem, scientific-QC, and artifact-integrity incidents prohibit retry until their contract failure is corrected. `internal_unknown` prohibits promotion and must be classified before another run is accepted.

### Gate Sequence

1. Preserve and link the accepted immutable-input, executor-parity, corrected Gate A–D, and Methx/Methrix evidence.
2. Complete the evidence register and final Gate 6 decision record.
3. Perform BS-PDX publication and artifact-manifest verification only if formal publication is required.
4. Keep the fresh seven-input matrix, representative repeats, scale qualification, WGBS requalification, and additional Snakemake recovery as deferred future extensions.

## 9. 实施检查清单

- [x] 6.1 — Benchmark plan + metrics schema
- [x] 6.2 — Failure injection tests (local)
- [x] 6.3 — Accepted bounded canary and executor-parity evidence for RRBS, RNA-seq, BS-PDX, and RNA-PDX
- [x] 6.4 — Corrected Gate A–D audits, Methx/Methrix CpG/matrix parity, and documented evidence boundaries
- [ ] 6.5 — Deferred: representative `20 samples × 3 runs` matrix
- [ ] 6.6 — Deferred: production throughput and scheduler-pressure qualification
- [x] 6.7 — Publish and link the Gate 6 closeout register and final scope record
- [ ] 6.8 — Conditional: BS-PDX publication and complete artifact-manifest verification if formal publication is required

Items 6.5 and 6.6 are explicitly non-blocking future qualification. WGBS requalification and additional Snakemake interruption/recovery comparison are also deferred and are not separate current checklist blockers.
