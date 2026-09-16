#!/usr/bin/env python3
"""Compare independent NM oracle rows with Xenofilx calculator audit rows."""

from __future__ import annotations

import argparse
import csv
import gzip
import json
from itertools import zip_longest
from pathlib import Path
from typing import TextIO

EXPECTED_ORACLE_HEADER = [
    "record_ordinal",
    "qname",
    "flag",
    "reference",
    "position_0_based",
    "cigar",
    "conventional_nm",
    "xenofilx_bisulfite_nm",
    "insertions",
    "deletions",
    "soft_clips",
    "bisulfite_conversions_ignored",
    "classification_score",
]
EXPECTED_XENOFILX_HEADER = [
    "record_ordinal",
    "qname",
    "flag",
    "reference",
    "position_0_based",
    "cigar",
    "stored_nm",
    "nm",
    "insertions",
    "soft_clips",
    "classification_score",
]


def open_text(path: Path) -> TextIO:
    if path.suffix == ".gz":
        return gzip.open(path, "rt", encoding="utf-8", newline="")
    return path.open("r", encoding="utf-8", newline="")


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--oracle", required=True, type=Path)
    parser.add_argument("--xenofilx", required=True, type=Path)
    parser.add_argument("--report", required=True, type=Path)
    parser.add_argument(
        "--xenofilx-mode",
        choices=("conventional", "bisulfite"),
        default="bisulfite",
        help="NM contract selected by the Xenofilx audit",
    )
    return parser.parse_args()


def compare_rows(
    oracle_path: Path,
    xenofilx_path: Path,
    xenofilx_mode: str,
) -> dict[str, object]:
    expected_nm_field = (
        "xenofilx_bisulfite_nm"
        if xenofilx_mode == "bisulfite"
        else "conventional_nm"
    )
    report: dict[str, object] = {
        "schema_version": "gate6.nm-score-comparison/v2",
        "oracle_path": str(oracle_path),
        "xenofilx_path": str(xenofilx_path),
        "xenofilx_mode": xenofilx_mode,
        "records_compared": 0,
        "equal": True,
        "first_difference": None,
        "difference_counts": {},
    }
    with open_text(oracle_path) as oracle_file, open_text(xenofilx_path) as xenofilx_file:
        oracle_reader = csv.DictReader(oracle_file, delimiter="\t")
        xenofilx_reader = csv.DictReader(xenofilx_file, delimiter="\t")
        if oracle_reader.fieldnames != EXPECTED_ORACLE_HEADER:
            raise ValueError(f"unexpected oracle header: {oracle_reader.fieldnames}")
        if xenofilx_reader.fieldnames != EXPECTED_XENOFILX_HEADER:
            raise ValueError(f"unexpected Xenofilx header: {xenofilx_reader.fieldnames}")

        for row_number, paired_rows in enumerate(zip_longest(oracle_reader, xenofilx_reader), start=1):
            oracle_row, xenofilx_row = paired_rows
            if oracle_row is None or xenofilx_row is None:
                register_difference(report, "record_count", row_number, oracle_row, xenofilx_row)
                break
            report["records_compared"] = row_number
            for field_name in ("record_ordinal", "qname", "flag", "reference", "position_0_based", "cigar"):
                if oracle_row[field_name] != xenofilx_row[field_name]:
                    register_difference(report, f"identity:{field_name}", row_number, oracle_row, xenofilx_row)
                    break
            expected_classification_score = int(oracle_row["classification_score"])
            if xenofilx_mode == "conventional":
                expected_classification_score += int(oracle_row["bisulfite_conversions_ignored"])
            for oracle_field, xenofilx_field in (
                (expected_nm_field, "nm"),
                ("insertions", "insertions"),
                ("soft_clips", "soft_clips"),
            ):
                if oracle_row[oracle_field] != xenofilx_row[xenofilx_field]:
                    register_difference(report, f"score:{oracle_field}", row_number, oracle_row, xenofilx_row)
                    break
            if expected_classification_score != int(xenofilx_row["classification_score"]):
                register_difference(report, "score:classification_score", row_number, oracle_row, xenofilx_row)

    report["difference_counts"] = dict(sorted(report["difference_counts"].items()))
    return report


def register_difference(
    report: dict[str, object],
    category: str,
    row_number: int,
    oracle_row: dict[str, str] | None,
    xenofilx_row: dict[str, str] | None,
) -> None:
    report["equal"] = False
    difference_counts = report["difference_counts"]
    assert isinstance(difference_counts, dict)
    difference_counts[category] = difference_counts.get(category, 0) + 1
    if report["first_difference"] is None:
        report["first_difference"] = {
            "row_number": row_number,
            "category": category,
            "oracle": oracle_row,
            "xenofilx": xenofilx_row,
        }


def main() -> int:
    arguments = parse_arguments()
    report = compare_rows(arguments.oracle, arguments.xenofilx, arguments.xenofilx_mode)
    arguments.report.parent.mkdir(parents=True, exist_ok=True)
    arguments.report.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    return 0 if report["equal"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
