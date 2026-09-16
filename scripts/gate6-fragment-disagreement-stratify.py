#!/usr/bin/env python3
"""Stratify deterministic Gate 6 modern-only fragment disagreement samples."""

from __future__ import annotations

import argparse
import csv
import gzip
import hashlib
import heapq
import json
import subprocess
from collections import Counter
from dataclasses import dataclass, field
from pathlib import Path
from typing import Iterable


@dataclass
class AlignmentProfile:
    total_records: int = 0
    mapped_primary_records: int = 0
    unmapped_records: int = 0
    secondary_records: int = 0
    supplementary_records: int = 0
    paired_records: int = 0
    first_mate_primary_records: int = 0
    second_mate_primary_records: int = 0
    invalid_primary_mate_records: int = 0

    def add_alignment(self, flag: int) -> None:
        self.total_records += 1
        if flag & 0x1:
            self.paired_records += 1
        if flag & 0x4:
            self.unmapped_records += 1
        if flag & 0x100:
            self.secondary_records += 1
        if flag & 0x800:
            self.supplementary_records += 1

        is_primary_mapped = not flag & (0x4 | 0x100 | 0x800)
        if not is_primary_mapped:
            return
        self.mapped_primary_records += 1
        is_first_mate = bool(flag & 0x40)
        is_second_mate = bool(flag & 0x80)
        if is_first_mate and not is_second_mate:
            self.first_mate_primary_records += 1
        elif is_second_mate and not is_first_mate:
            self.second_mate_primary_records += 1
        else:
            self.invalid_primary_mate_records += 1

    def category(self) -> str:
        if self.mapped_primary_records == 0:
            return "none"
        if self.invalid_primary_mate_records:
            return "invalid_mate_flag"
        if self.first_mate_primary_records > 1 or self.second_mate_primary_records > 1:
            return "duplicate_primary_mate"
        if self.first_mate_primary_records == 1 and self.second_mate_primary_records == 1:
            return "complete_pair"
        if self.first_mate_primary_records == 1:
            return "first_mate_only"
        if self.second_mate_primary_records == 1:
            return "second_mate_only"
        return "unpaired_primary"

    def as_dict(self) -> dict[str, int | str]:
        return {
            "category": self.category(),
            "total_records": self.total_records,
            "mapped_primary_records": self.mapped_primary_records,
            "unmapped_records": self.unmapped_records,
            "secondary_records": self.secondary_records,
            "supplementary_records": self.supplementary_records,
            "paired_records": self.paired_records,
            "first_mate_primary_records": self.first_mate_primary_records,
            "second_mate_primary_records": self.second_mate_primary_records,
            "invalid_primary_mate_records": self.invalid_primary_mate_records,
        }


@dataclass
class FragmentEvidence:
    original_graft: AlignmentProfile = field(default_factory=AlignmentProfile)
    original_host: AlignmentProfile = field(default_factory=AlignmentProfile)
    modern_filtered: AlignmentProfile = field(default_factory=AlignmentProfile)
    legacy_filtered: AlignmentProfile = field(default_factory=AlignmentProfile)


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--samtools", required=True, type=Path)
    parser.add_argument("--disagreements", required=True, type=Path)
    parser.add_argument("--original-graft", required=True, type=Path)
    parser.add_argument("--original-host", required=True, type=Path)
    parser.add_argument("--modern-filtered", required=True, type=Path)
    parser.add_argument("--legacy-filtered", required=True, type=Path)
    parser.add_argument("--report", required=True, type=Path)
    parser.add_argument("--samples", required=True, type=Path)
    parser.add_argument("--sample-size", default=10000, type=int)
    parser.add_argument("--classification", default="graft/discarded")
    parser.add_argument("--seed", default="gate6-strand-aware-v1")
    return parser.parse_args()


def select_deterministic_sample(
    disagreements_path: Path,
    classification: str,
    sample_size: int,
    seed: str,
) -> tuple[set[str], int]:
    if sample_size <= 0:
        raise ValueError("--sample-size must be positive")

    retained: list[tuple[bytes, str]] = []
    matching_disagreements = 0
    with gzip.open(disagreements_path, "rt", encoding="utf-8", newline="") as disagreement_file:
        reader = csv.DictReader(disagreement_file, delimiter="\t")
        expected_header = ["fragment_id", "modern_classification", "legacy_classification"]
        if reader.fieldnames != expected_header:
            raise ValueError(f"unexpected disagreement header: {reader.fieldnames}")
        for row in reader:
            observed_classification = f"{row['modern_classification']}/{row['legacy_classification']}"
            if observed_classification != classification:
                continue
            matching_disagreements += 1
            fragment_id = row["fragment_id"]
            sample_key = hashlib.blake2b(
                f"{seed}\0{fragment_id}".encode("utf-8"), digest_size=16
            ).digest()
            heap_item = (bytes(~value & 0xFF for value in sample_key), fragment_id)
            if len(retained) < sample_size:
                heapq.heappush(retained, heap_item)
            elif heap_item > retained[0]:
                heapq.heapreplace(retained, heap_item)

    selected = {fragment_id for _, fragment_id in retained}
    if not selected:
        raise ValueError(f"no disagreements matched classification {classification!r}")
    return selected, matching_disagreements


def stream_bam_profiles(
    samtools_path: Path,
    bam_path: Path,
    target_fragments: set[str],
    profile_name: str,
    evidence_by_fragment: dict[str, FragmentEvidence],
) -> None:
    process = subprocess.Popen(
        [str(samtools_path), "view", str(bam_path)],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
    )
    assert process.stdout is not None
    for line in process.stdout:
        fields = line.rstrip("\n").split("\t")
        if len(fields) < 11:
            raise ValueError(f"malformed SAM record from {bam_path}: {line!r}")
        fragment_id = fields[0]
        if fragment_id not in target_fragments:
            continue
        getattr(evidence_by_fragment[fragment_id], profile_name).add_alignment(int(fields[1]))
    process.stdout.close()
    stderr = process.stderr.read() if process.stderr is not None else ""
    if process.wait() != 0:
        raise RuntimeError(f"samtools view failed for {bam_path}: {stderr.strip()}")


def write_sample_rows(samples_path: Path, evidence_by_fragment: dict[str, FragmentEvidence]) -> None:
    samples_path.parent.mkdir(parents=True, exist_ok=True)
    with samples_path.open("w", encoding="utf-8", newline="") as samples_file:
        fieldnames = [
            "fragment_id",
            "original_graft_category",
            "original_host_category",
            "modern_filtered_category",
            "legacy_filtered_category",
            "original_graft_primary_records",
            "original_host_primary_records",
            "modern_filtered_primary_records",
            "legacy_filtered_primary_records",
        ]
        writer = csv.DictWriter(samples_file, fieldnames=fieldnames, delimiter="\t")
        writer.writeheader()
        for fragment_id, evidence in sorted(evidence_by_fragment.items()):
            writer.writerow(
                {
                    "fragment_id": fragment_id,
                    "original_graft_category": evidence.original_graft.category(),
                    "original_host_category": evidence.original_host.category(),
                    "modern_filtered_category": evidence.modern_filtered.category(),
                    "legacy_filtered_category": evidence.legacy_filtered.category(),
                    "original_graft_primary_records": evidence.original_graft.mapped_primary_records,
                    "original_host_primary_records": evidence.original_host.mapped_primary_records,
                    "modern_filtered_primary_records": evidence.modern_filtered.mapped_primary_records,
                    "legacy_filtered_primary_records": evidence.legacy_filtered.mapped_primary_records,
                }
            )


def summarize_profiles(evidence_by_fragment: Iterable[FragmentEvidence]) -> dict[str, dict[str, int]]:
    counters = {
        "original_graft": Counter(),
        "original_host": Counter(),
        "modern_filtered": Counter(),
        "legacy_filtered": Counter(),
        "joint_original_alignment_shape": Counter(),
        "output_membership": Counter(),
    }
    for evidence in evidence_by_fragment:
        counters["original_graft"][evidence.original_graft.category()] += 1
        counters["original_host"][evidence.original_host.category()] += 1
        counters["modern_filtered"][evidence.modern_filtered.category()] += 1
        counters["legacy_filtered"][evidence.legacy_filtered.category()] += 1
        counters["joint_original_alignment_shape"][
            f"graft={evidence.original_graft.category()};host={evidence.original_host.category()}"
        ] += 1
        counters["output_membership"][
            f"modern={evidence.modern_filtered.category()};legacy={evidence.legacy_filtered.category()}"
        ] += 1
    return {name: dict(sorted(counter.items())) for name, counter in counters.items()}


def main() -> int:
    arguments = parse_arguments()
    required_paths = (
        arguments.samtools,
        arguments.disagreements,
        arguments.original_graft,
        arguments.original_host,
        arguments.modern_filtered,
        arguments.legacy_filtered,
    )
    missing_paths = [str(path) for path in required_paths if not path.is_file()]
    if missing_paths:
        raise ValueError(f"missing required regular files: {', '.join(missing_paths)}")

    selected_fragments, matching_disagreements = select_deterministic_sample(
        arguments.disagreements,
        arguments.classification,
        arguments.sample_size,
        arguments.seed,
    )
    evidence_by_fragment = {
        fragment_id: FragmentEvidence() for fragment_id in selected_fragments
    }
    for profile_name, bam_path in (
        ("original_graft", arguments.original_graft),
        ("original_host", arguments.original_host),
        ("modern_filtered", arguments.modern_filtered),
        ("legacy_filtered", arguments.legacy_filtered),
    ):
        stream_bam_profiles(
            arguments.samtools,
            bam_path,
            selected_fragments,
            profile_name,
            evidence_by_fragment,
        )

    write_sample_rows(arguments.samples, evidence_by_fragment)
    report = {
        "schema_version": "gate6.fragment-disagreement-stratification/v1",
        "classification": arguments.classification,
        "matching_disagreements": matching_disagreements,
        "sample_size": len(selected_fragments),
        "seed": arguments.seed,
        "input_paths": {
            "disagreements": str(arguments.disagreements),
            "original_graft": str(arguments.original_graft),
            "original_host": str(arguments.original_host),
            "modern_filtered": str(arguments.modern_filtered),
            "legacy_filtered": str(arguments.legacy_filtered),
        },
        "categories": summarize_profiles(evidence_by_fragment.values()),
    }
    arguments.report.parent.mkdir(parents=True, exist_ok=True)
    arguments.report.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    expected_modern_only = all(
        evidence.modern_filtered.mapped_primary_records > 0
        and evidence.legacy_filtered.mapped_primary_records == 0
        for evidence in evidence_by_fragment.values()
    )
    return 0 if expected_modern_only else 1


if __name__ == "__main__":
    raise SystemExit(main())
