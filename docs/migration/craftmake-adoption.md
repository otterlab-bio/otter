# Craftmake adoption roadmap

Craftmake is the native execution layer for Otter. Adoption is deliberately dual-track: established production assets continue to use the explicit Snakemake compatibility path while Craftmake evidence is expanded and reviewed.

## Current decision

- Craftmake is the canonical executor for resolved v1 runs.
- Snakemake remains an explicit compatibility executor for legacy project layouts and established assets.
- A Craftmake failure never falls back to Snakemake.
- Building Craftmake or passing a local contract test does not prove universal production replacement.
- Gate 6 accepted a bounded executor comparison, corrected classification evidence, and Methx/Methrix parity.

## Gate status

| Gate | Scope | Status |
| --- | --- | --- |
| Gate 0 | Contract, schema, catalog, and evidence policy | Implemented documentation and schemas |
| Gate 1 | Typed project/run/reference config and resolver | Implemented; legacy adapter retained |
| Gate 2 | Executor routing, snapshot boundary, protocol, task correlation | Implemented in local contract scope |
| Gate 3 | Site/backend detection and resource preflight | Implemented locally; real workflow evidence remains bounded |
| Gate 4 | Immutable reference registry, locks, override, promotion | Implemented; production evidence is separately recorded |
| Gate 5 | Five scenario workflow catalog and publishers | Implemented as catalog/assets with local contract coverage |
| Gate 6 | Real-SLURM parity, recovery, scientific comparison, scale | Bounded scope accepted; deferred extensions remain |
| Gate 7 | Default stability and compatibility retirement decision | Not a side effect of this documentation or build work |

## What is canonical now

A new reproducible run should use:

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

The resolved snapshot is the contract. It binds scenario, executor, backend, toolchain, references, inputs, resources, workflow assets, and digests.

## What remains compatibility-only

Established projects using `otter.yaml` continue through an explicit Snakemake route:

```bash
otter run \
  --config my_project/userspace/<jobid>/config/otter.yaml \
  --executor snakemake \
  --engine slurm
```

The compatibility route is not a failure fallback and is not silently selected from a Craftmake error.

## Gate 6 deferred extensions

These items are intentionally outside the accepted current release evidence:

- fresh seven-input legacy-equivalent scientific matrix;
- representative `20 samples × 3 repeats` matrix;
- production-scale throughput and scheduler-pressure qualification;
- WGBS `SRR6373947` requalification;
- additional Snakemake interruption/recovery studies.

They may be scheduled as separate qualification work. They must not be described as completed or used as an implicit release blocker unless the release decision explicitly adopts them.

## Adoption requirements before a retirement decision

A future decision to retire Snakemake must be a separate change with:

1. accepted scenario-specific scientific and structural parity;
2. real Local/SLURM behavior and recovery evidence;
3. documented migration and rollback procedures;
4. production incident ownership and support runbooks;
5. an explicit compatibility removal announcement.

Until then, documentation must use `dual-track`, `compatibility`, or `migration in progress` language.

[Back to the documentation hub](../README.md)
