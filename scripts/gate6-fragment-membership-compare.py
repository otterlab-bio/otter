#!/usr/bin/env python3
"""Compare selected-graft fragment membership from modern and legacy filters."""

from __future__ import annotations

import argparse
import gzip
import hashlib
import json
import os
import subprocess
import tempfile
from collections import Counter
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Iterator


@dataclass
class ComparisonSummary:
    schema_version: str
    original_graft_bam: str
    modern_filtered_bam: str
    legacy_filtered_bam: str
    total_input_fragments: int
    modern_graft_fragments: int
    legacy_graft_fragments: int
    agreements: dict[str, int]
    membership_disagreements: dict[str, int]
    disagreements: int
    modern_only_graft: int
    legacy_only_graft: int
    selected_subset_failures: dict[str, int]
    original_fragment_digest: str
    modern_fragment_digest: str
    legacy_fragment_digest: str
    disagreements_path: str


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Compare modern and legacy filtered BAM fragment membership."
    )
    parser.add_argument("--samtools", required=True, help="samtools executable")
    parser.add_argument("--original-graft", required=True, help="original graft BAM")
    parser.add_argument("--modern-filtered", required=True, help="modern filtered graft BAM")
    parser.add_argument("--legacy-filtered", required=True, help="legacy filtered graft BAM")
    parser.add_argument("--report", required=True, help="output JSON report")
    parser.add_argument(
        "--disagreements",
        required=True,
        help="output gzip TSV containing modern/legacy selection disagreements",
    )
    parser.add_argument(
        "--temporary-directory",
        required=True,
        help="existing writable directory for externally sorted QNAME files",
    )
    return parser.parse_args()


def require_readable_file(path: Path) -> None:
    if not path.is_file():
        raise ValueError(f"required regular file does not exist: {path}")


def write_sorted_unique_names(
    samtools_path: str, bam_path: Path, output_path: Path, temporary_directory: Path
) -> None:
    view_process = subprocess.Popen(
        [samtools_path, "view", bam_path.as_posix()],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if view_process.stdout is None:
        raise RuntimeError(f"could not read BAM stream for {bam_path}")

    sort_process = subprocess.Popen(
        [
            "sort",
            "--unique",
            "--temporary-directory",
            temporary_directory.as_posix(),
            "--output",
            output_path.as_posix(),
        ],
        stdin=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        env={**os.environ, "LC_ALL": "C"},
    )
    if sort_process.stdin is None:
        view_process.kill()
        raise RuntimeError(f"could not open QNAME sorter for {bam_path}")

    try:
        for alignment_line in view_process.stdout:
            qname, separator, _ = alignment_line.partition("\t")
            if not separator or not qname:
                raise ValueError(f"malformed SAM record while reading {bam_path}")
            sort_process.stdin.write(qname)
            sort_process.stdin.write("\n")
    finally:
        sort_process.stdin.close()
        view_process.stdout.close()

    view_stderr = view_process.stderr.read() if view_process.stderr else ""
    sort_stderr = sort_process.stderr.read() if sort_process.stderr else ""
    view_status = view_process.wait()
    sort_status = sort_process.wait()
    if view_status != 0:
        raise RuntimeError(f"samtools view failed for {bam_path}: {view_stderr.strip()}")
    if sort_status != 0:
        raise RuntimeError(f"sort failed for {bam_path}: {sort_stderr.strip()}")


def iterate_names(path: Path) -> Iterator[str]:
    with path.open("rt", encoding="utf-8", newline="") as input_file:
        for line in input_file:
            name = line.rstrip("\n")
            if not name:
                raise ValueError(f"empty QNAME in sorted fragment list: {path}")
            yield name


def get_next_name(names: Iterator[str]) -> str | None:
    return next(names, None)


def calculate_digest(path: Path) -> tuple[int, str]:
    digest = hashlib.sha256()
    name_count = 0
    with path.open("rb") as input_file:
        for name_line in input_file:
            digest.update(name_line)
            name_count += 1
    return name_count, digest.hexdigest()


def compare_fragment_membership(
    original_names_path: Path,
    modern_names_path: Path,
    legacy_names_path: Path,
    disagreements_path: Path,
) -> ComparisonSummary:
    total_input_fragments, original_digest = calculate_digest(original_names_path)
    modern_graft_fragments, modern_digest = calculate_digest(modern_names_path)
    legacy_graft_fragments, legacy_digest = calculate_digest(legacy_names_path)
    agreements: Counter[str] = Counter()
    membership_disagreements: Counter[str] = Counter()
    selected_subset_failures: Counter[str] = Counter()
    disagreements = 0

    modern_names = iterate_names(modern_names_path)
    legacy_names = iterate_names(legacy_names_path)
    modern_name = get_next_name(modern_names)
    legacy_name = get_next_name(legacy_names)

    with gzip.open(disagreements_path, "wt", encoding="utf-8", newline="") as output_file:
        output_file.write("fragment_id\tmodern_classification\tlegacy_classification\n")
        for original_name in iterate_names(original_names_path):
            while modern_name is not None and modern_name < original_name:
                selected_subset_failures["modern_selected_not_in_original"] += 1
                modern_name = get_next_name(modern_names)
            while legacy_name is not None and legacy_name < original_name:
                selected_subset_failures["legacy_selected_not_in_original"] += 1
                legacy_name = get_next_name(legacy_names)

            modern_classification = "graft" if modern_name == original_name else "discarded"
            legacy_classification = "graft" if legacy_name == original_name else "discarded"
            if modern_name == original_name:
                modern_name = get_next_name(modern_names)
            if legacy_name == original_name:
                legacy_name = get_next_name(legacy_names)

            if modern_classification == legacy_classification:
                agreements[f"{modern_classification}/{legacy_classification}"] += 1
                continue
            disagreements += 1
            disagreement_key = f"{modern_classification}/{legacy_classification}"
            membership_disagreements[disagreement_key] += 1
            output_file.write(
                f"{original_name}\t{modern_classification}\t{legacy_classification}\n"
            )

        while modern_name is not None:
            selected_subset_failures["modern_selected_not_in_original"] += 1
            modern_name = get_next_name(modern_names)
        while legacy_name is not None:
            selected_subset_failures["legacy_selected_not_in_original"] += 1
            legacy_name = get_next_name(legacy_names)

    return ComparisonSummary(
        schema_version="gate6.fragment-membership-comparison/v1",
        original_graft_bam="",
        modern_filtered_bam="",
        legacy_filtered_bam="",
        total_input_fragments=total_input_fragments,
        modern_graft_fragments=modern_graft_fragments,
        legacy_graft_fragments=legacy_graft_fragments,
        agreements=dict(sorted(agreements.items())),
        membership_disagreements=dict(sorted(membership_disagreements.items())),
        disagreements=disagreements,
        modern_only_graft=membership_disagreements["graft/discarded"],
        legacy_only_graft=membership_disagreements["discarded/graft"],
        selected_subset_failures=dict(sorted(selected_subset_failures.items())),
        original_fragment_digest=original_digest,
        modern_fragment_digest=modern_digest,
        legacy_fragment_digest=legacy_digest,
        disagreements_path=disagreements_path.as_posix(),
    )


def main() -> None:
    arguments = parse_arguments()
    original_graft_bam = Path(arguments.original_graft).absolute()
    modern_filtered_bam = Path(arguments.modern_filtered).absolute()
    legacy_filtered_bam = Path(arguments.legacy_filtered).absolute()
    report_path = Path(arguments.report).resolve()
    disagreements_path = Path(arguments.disagreements).resolve()
    temporary_directory = Path(arguments.temporary_directory).resolve()
    for input_path in (original_graft_bam, modern_filtered_bam, legacy_filtered_bam):
        require_readable_file(input_path)
    if not temporary_directory.is_dir():
        raise ValueError(f"temporary directory does not exist: {temporary_directory}")
    report_path.parent.mkdir(parents=True, exist_ok=True)
    disagreements_path.parent.mkdir(parents=True, exist_ok=True)

    with tempfile.TemporaryDirectory(dir=temporary_directory) as sorting_directory_string:
        sorting_directory = Path(sorting_directory_string)
        original_names_path = sorting_directory / "original-graft.names.txt"
        modern_names_path = sorting_directory / "modern-filtered.names.txt"
        legacy_names_path = sorting_directory / "legacy-filtered.names.txt"
        write_sorted_unique_names(arguments.samtools, original_graft_bam, original_names_path, sorting_directory)
        write_sorted_unique_names(arguments.samtools, modern_filtered_bam, modern_names_path, sorting_directory)
        write_sorted_unique_names(arguments.samtools, legacy_filtered_bam, legacy_names_path, sorting_directory)
        summary = compare_fragment_membership(
            original_names_path, modern_names_path, legacy_names_path, disagreements_path
        )

    summary.original_graft_bam = original_graft_bam.as_posix()
    summary.modern_filtered_bam = modern_filtered_bam.as_posix()
    summary.legacy_filtered_bam = legacy_filtered_bam.as_posix()
    report_path.write_text(json.dumps(asdict(summary), indent=2) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
