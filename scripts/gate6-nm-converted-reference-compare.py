#!/usr/bin/env python3
"""Compare original Xenofilx bisulfite NM with CT/GA reference controls."""

from __future__ import annotations

import argparse
import csv
import json
import subprocess
from collections import Counter
from pathlib import Path


IDENTITY_FIELDS = ("qname", "flag", "reference", "position_0_based", "cigar")


def parse_labeled_paths(value: str) -> tuple[str, tuple[Path, Path, Path, Path]]:
    label, separator, path_values = value.partition("=")
    paths = tuple(Path(path_value) for path_value in path_values.split(","))
    if not separator or not label or len(paths) != 4 or any(not path_value for path_value in paths):
        raise argparse.ArgumentTypeError(
            "--control must be LABEL=BAM,ORACLE,XENOFILX,PICARD_BAM"
        )
    return label, paths


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--samtools", required=True, type=Path)
    parser.add_argument("--original-bam", required=True, type=Path)
    parser.add_argument("--original-oracle", required=True, type=Path)
    parser.add_argument("--original-xenofilx", required=True, type=Path)
    parser.add_argument(
        "--control",
        action="append",
        required=True,
        type=parse_labeled_paths,
        metavar="LABEL=BAM,ORACLE,XENOFILX,PICARD_BAM",
    )
    parser.add_argument("--records-read", required=True, type=int)
    parser.add_argument("--report", required=True, type=Path)
    return parser.parse_args()


def read_tsv(path: Path) -> dict[int, dict[str, str]]:
    with path.open("r", encoding="utf-8", newline="") as report_file:
        return {
            int(row["record_ordinal"]): row
            for row in csv.DictReader(report_file, delimiter="\t")
        }


def alignment_identity(fields: list[str]) -> tuple[str, str, str, str, str]:
    return fields[0], fields[1], fields[2], str(int(fields[3]) - 1), fields[5]


def read_mapped_prefix(
    samtools_path: Path,
    bam_path: Path,
    records_read: int,
) -> list[dict[str, str]]:
    process = subprocess.Popen(
        [str(samtools_path), "view", str(bam_path)],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
    )
    assert process.stdout is not None
    records: list[dict[str, str]] = []
    occurrence_counts: Counter[tuple[str, str, str, str, str]] = Counter()
    for record_ordinal, line in enumerate(process.stdout):
        if record_ordinal >= records_read:
            break
        fields = line.rstrip("\n").split("\t")
        if len(fields) < 11:
            raise ValueError(f"malformed SAM record in {bam_path}: {line!r}")
        if int(fields[1]) & 4:
            continue
        identity = alignment_identity(fields)
        occurrence = occurrence_counts[identity]
        occurrence_counts[identity] += 1
        records.append(
            {
                "record_ordinal": str(record_ordinal),
                **dict(zip(IDENTITY_FIELDS, identity, strict=True)),
                "occurrence": str(occurrence),
            }
        )
    process.stdout.close()
    stderr = process.stderr.read() if process.stderr is not None else ""
    return_code = process.wait()
    if return_code not in (0, -13):
        raise RuntimeError(f"samtools view failed for {bam_path}: {stderr.strip()}")
    return records


def read_control_prefix(
    samtools_path: Path,
    bam_path: Path,
    audit_rows: dict[int, dict[str, str]],
    target_records: dict[tuple[str, str, str, str, str], list[dict[str, str]]],
) -> dict[str, dict[str, str]]:
    process = subprocess.Popen(
        [str(samtools_path), "view", str(bam_path)],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
    )
    assert process.stdout is not None
    maximum_ordinal = max(audit_rows, default=-1)
    matched_records: dict[str, dict[str, str]] = {}
    consumed_candidates: Counter[tuple[str, str, str, str, str]] = Counter()
    for record_ordinal, line in enumerate(process.stdout):
        if record_ordinal > maximum_ordinal:
            break
        fields = line.rstrip("\n").split("\t")
        if len(fields) < 11 or int(fields[1]) & 4:
            continue
        identity = alignment_identity(fields)
        candidates = target_records.get(identity)
        if not candidates or record_ordinal not in audit_rows:
            continue
        candidate_index = consumed_candidates[identity]
        if candidate_index >= len(candidates):
            continue
        consumed_candidates[identity] += 1
        original_record = candidates[candidate_index]
        matched_records[original_record["record_ordinal"]] = audit_rows[record_ordinal]
    process.stdout.close()
    stderr = process.stderr.read() if process.stderr is not None else ""
    return_code = process.wait()
    if return_code not in (0, -13):
        raise RuntimeError(f"samtools view failed for {bam_path}: {stderr.strip()}")
    return matched_records


def extract_nm_tag(fields: list[str]) -> int:
    for field in fields[11:]:
        if field.startswith("NM:i:"):
            return int(field[5:])
    return -1


def read_picard_for_controls(
    samtools_path: Path,
    bam_path: Path,
    target_records: dict[tuple[str, str, str, str, str], list[dict[str, str]]],
) -> dict[str, int]:
    process = subprocess.Popen(
        [str(samtools_path), "view", str(bam_path)],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
    )
    assert process.stdout is not None
    matched_records: dict[str, int] = {}
    consumed_candidates: Counter[tuple[str, str, str, str, str]] = Counter()
    for line in process.stdout:
        fields = line.rstrip("\n").split("\t")
        if len(fields) < 11 or int(fields[1]) & 4:
            continue
        identity = alignment_identity(fields)
        candidates = target_records.get(identity)
        if not candidates:
            continue
        candidate_index = consumed_candidates[identity]
        if candidate_index >= len(candidates):
            continue
        consumed_candidates[identity] += 1
        original_record = candidates[candidate_index]
        matched_records[original_record["record_ordinal"]] = extract_nm_tag(fields)
    process.stdout.close()
    stderr = process.stderr.read() if process.stderr is not None else ""
    return_code = process.wait()
    if return_code not in (0, -13):
        raise RuntimeError(f"samtools view failed for {bam_path}: {stderr.strip()}")
    return matched_records


def compare_control(
    label: str,
    control_paths: tuple[Path, Path, Path, Path],
    arguments: argparse.Namespace,
    original_records: list[dict[str, str]],
    original_oracle_rows: dict[int, dict[str, str]],
    original_xenofilx_rows: dict[int, dict[str, str]],
) -> dict[str, object]:
    control_bam, control_oracle_path, control_xenofilx_path, picard_bam = control_paths
    control_oracle_rows = read_tsv(control_oracle_path)
    control_xenofilx_rows = read_tsv(control_xenofilx_path)
    target_records: dict[tuple[str, str, str, str, str], list[dict[str, str]]] = {}
    for record in original_records:
        identity = tuple(record[field_name] for field_name in IDENTITY_FIELDS)
        target_records.setdefault(identity, []).append(record)

    control_oracles = read_control_prefix(
        arguments.samtools, control_bam, control_oracle_rows, target_records
    )
    control_xenofilx = read_control_prefix(
        arguments.samtools, control_bam, control_xenofilx_rows, target_records
    )
    picard_nms = read_picard_for_controls(arguments.samtools, picard_bam, target_records)

    differences = Counter()
    compared_records = 0
    control_record_ordinals = sorted(control_oracles, key=int)
    for ordinal in control_record_ordinals:
        original_oracle = original_oracle_rows.get(int(ordinal))
        original_xenofilx = original_xenofilx_rows.get(int(ordinal))
        control_oracle = control_oracles[ordinal]
        control_xenofilx_row = control_xenofilx.get(ordinal)
        picard_nm = picard_nms.get(ordinal)
        if None in (original_oracle, original_xenofilx, control_xenofilx_row, picard_nm):
            differences["missing_corresponding_record"] += 1
            continue
        compared_records += 1
        expected_nm = int(original_oracle["xenofilx_bisulfite_nm"])
        if int(original_xenofilx["nm"]) != expected_nm:
            differences["original_xenofilx_vs_original_oracle"] += 1
        if int(control_oracle["conventional_nm"]) != expected_nm:
            differences["converted_reference_oracle_vs_original_bisulfite"] += 1
        if int(control_xenofilx_row["nm"]) != expected_nm:
            differences["converted_reference_xenofilx_vs_original_bisulfite"] += 1
        if picard_nm != expected_nm:
            differences["picard_vs_original_bisulfite"] += 1

    return {
        "label": label,
        "source_audit_records": len(original_records),
        "control_records": len(control_record_ordinals),
        "records_compared": compared_records,
        "difference_counts": dict(sorted(differences.items())),
        "equal_to_original_bisulfite": not differences,
    }


def main() -> int:
    arguments = parse_arguments()
    if arguments.records_read <= 0:
        raise ValueError("--records-read must be positive")
    controls = dict(arguments.control)
    if len(controls) != len(arguments.control):
        raise ValueError("control labels must be unique")

    original_records = read_mapped_prefix(
        arguments.samtools, arguments.original_bam, arguments.records_read
    )
    original_oracle_rows = read_tsv(arguments.original_oracle)
    original_xenofilx_rows = read_tsv(arguments.original_xenofilx)
    report = {
        "schema_version": "gate6.converted-reference-control/v1",
        "original_bam": str(arguments.original_bam),
        "records_read": arguments.records_read,
        "controls": [
            compare_control(
                label,
                control_paths,
                arguments,
                original_records,
                original_oracle_rows,
                original_xenofilx_rows,
            )
            for label, control_paths in controls.items()
        ],
    }
    arguments.report.parent.mkdir(parents=True, exist_ok=True)
    arguments.report.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
