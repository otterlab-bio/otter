#!/usr/bin/env python3
"""Compare per-read Picard NM tags with independent and Xenofilx audits."""

from __future__ import annotations

import argparse
import csv
import json
import subprocess
from collections import Counter, defaultdict
from pathlib import Path
from typing import TextIO


IDENTITY_FIELDS = ("qname", "flag", "reference", "position_0_based", "cigar")


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--samtools", required=True, type=Path)
    parser.add_argument("--original-bam", required=True, type=Path)
    parser.add_argument(
        "--picard-bam",
        required=True,
        action="append",
        type=parse_labeled_path,
        metavar="LABEL=PATH",
        help="repeatable Picard output; labels must be unique",
    )
    parser.add_argument("--oracle", required=True, type=Path)
    parser.add_argument("--xenofilx", required=True, type=Path)
    parser.add_argument(
        "--records-read",
        required=True,
        type=int,
        help="raw-record prefix used to define the audit universe",
    )
    parser.add_argument("--xenofilx-mode", choices=("conventional", "bisulfite"), required=True)
    parser.add_argument("--report", required=True, type=Path)
    parser.add_argument("--differences", required=True, type=Path)
    return parser.parse_args()


def parse_labeled_path(value: str) -> tuple[str, Path]:
    label, separator, path_value = value.partition("=")
    if not separator or not label or not path_value:
        raise argparse.ArgumentTypeError("--picard-bam must be LABEL=PATH")
    return label, Path(path_value)


def read_audit_rows(path: Path) -> dict[int, dict[str, str]]:
    with path.open("r", encoding="utf-8", newline="") as report_file:
        rows = csv.DictReader(report_file, delimiter="\t")
        return {int(row["record_ordinal"]): row for row in rows}


def alignment_base_identity(fields: list[str]) -> tuple[str, str, str, str, str]:
    return fields[0], fields[1], fields[2], str(int(fields[3]) - 1), fields[5]


def record_base_identity(record: dict[str, str]) -> tuple[str, str, str, str, str]:
    return tuple(record[field_name] for field_name in IDENTITY_FIELDS)


def occurrence_key(
    base_identity: tuple[str, str, str, str, str],
    occurrence: int,
) -> tuple[str, str, str, str, str, int]:
    return (*base_identity, occurrence)


def read_original_mapped_records(
    samtools: Path,
    bam_path: Path,
    records_read: int,
) -> list[dict[str, str]]:
    process = subprocess.Popen(
        [str(samtools), "view", str(bam_path)],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
    )
    assert process.stdout is not None
    mapped_records: list[dict[str, str]] = []
    occurrence_counts: Counter[tuple[str, str, str, str, str]] = Counter()
    for record_ordinal, line in enumerate(process.stdout):
        if record_ordinal >= records_read:
            break
        fields = line.rstrip("\n").split("\t")
        if len(fields) < 11:
            raise ValueError(f"malformed SAM record in {bam_path}: {line!r}")
        if int(fields[1]) & 4:
            continue
        base_identity = alignment_base_identity(fields)
        occurrence = occurrence_counts[base_identity]
        occurrence_counts[base_identity] += 1
        mapped_records.append(
            {
                "record_ordinal": str(record_ordinal),
                "qname": fields[0],
                "flag": fields[1],
                "reference": fields[2],
                "position_0_based": str(int(fields[3]) - 1),
                "cigar": fields[5],
                "occurrence": str(occurrence),
                "identity_key": json.dumps(occurrence_key(base_identity, occurrence)),
            }
        )
    process.stdout.close()
    stderr = process.stderr.read() if process.stderr is not None else ""
    return_code = process.wait()
    if return_code not in (0, -13):
        raise RuntimeError(f"samtools view failed for {bam_path}: {stderr.strip()}")
    return mapped_records


def read_picard_nm_for_universe(
    samtools: Path,
    bam_path: Path,
    universe: dict[str, dict[str, str]],
) -> dict[str, dict[str, str]]:
    process = subprocess.Popen(
        [str(samtools), "view", str(bam_path)],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
    )
    assert process.stdout is not None
    matched_records: dict[str, dict[str, str]] = {}
    target_base_identities = {
        record_base_identity(record) for record in universe.values()
    }
    occurrence_counts: Counter[tuple[str, str, str, str, str]] = Counter()
    for line in process.stdout:
        fields = line.rstrip("\n").split("\t")
        if len(fields) < 11 or int(fields[1]) & 4:
            continue
        base_identity = alignment_base_identity(fields)
        if base_identity not in target_base_identities:
            continue
        occurrence = occurrence_counts[base_identity]
        occurrence_counts[base_identity] += 1
        key = json.dumps(occurrence_key(base_identity, occurrence))
        if key not in universe:
            continue
        matched_records[key] = {
            "qname": fields[0],
            "flag": fields[1],
            "reference": fields[2],
            "position_0_based": str(int(fields[3]) - 1),
            "cigar": fields[5],
            "occurrence": str(occurrence),
            "stored_nm": extract_nm_tag(fields[11:]),
        }
    process.stdout.close()
    stderr = process.stderr.read() if process.stderr is not None else ""
    return_code = process.wait()
    if return_code not in (0, -13):
        raise RuntimeError(f"samtools view failed for {bam_path}: {stderr.strip()}")
    return matched_records


def extract_nm_tag(auxiliary_fields: list[str]) -> str:
    for auxiliary_field in auxiliary_fields:
        if auxiliary_field.startswith("NM:i:"):
            return auxiliary_field[5:]
    return "-1"


def add_difference(
    difference_counter: Counter[str],
    differences_file: TextIO,
    difference_record: dict[str, object],
    first_difference: dict[str, object] | None,
) -> dict[str, object] | None:
    for category in difference_record["categories"]:
        difference_counter[str(category)] += 1
    differences_file.write(json.dumps(difference_record, separators=(",", ":")) + "\n")
    return first_difference if first_difference is not None else difference_record


def compare_records(
    original_records: list[dict[str, str]],
    picard_records_by_label: dict[str, dict[str, dict[str, str]]],
    oracle_rows: dict[int, dict[str, str]],
    xenofilx_rows: dict[int, dict[str, str]],
    xenofilx_mode: str,
    differences_file: TextIO,
) -> dict[str, object]:
    expected_xenofilx_oracle_field = (
        "xenofilx_bisulfite_nm" if xenofilx_mode == "bisulfite" else "conventional_nm"
    )
    difference_counter: Counter[str] = Counter()
    delta_counts: dict[str, Counter[str]] = defaultdict(Counter)
    first_difference: dict[str, object] | None = None
    observed_picard_counts: Counter[str] = Counter()
    conventional_match_counts: Counter[str] = Counter()
    bisulfite_match_counts: Counter[str] = Counter()

    universe = {record["identity_key"]: record for record in original_records}
    for record in original_records:
        record_ordinal = int(record["record_ordinal"])
        oracle = oracle_rows.get(record_ordinal)
        xenofilx = xenofilx_rows.get(record_ordinal)
        categories: list[str] = []
        if oracle is None or xenofilx is None:
            categories.append("audit_record_missing")
        else:
            for field_name in IDENTITY_FIELDS:
                if oracle.get(field_name) != xenofilx.get(field_name):
                    categories.append(f"xenofilx_oracle_identity:{field_name}")
            if int(xenofilx["nm"]) != int(oracle[expected_xenofilx_oracle_field]):
                categories.append("xenofilx_vs_oracle_selected_nm")
            if int(xenofilx["nm"]) != int(oracle["conventional_nm"]):
                difference_counter["xenofilx_vs_oracle_conventional_nm"] += 1
            if int(xenofilx["nm"]) != int(oracle["xenofilx_bisulfite_nm"]):
                difference_counter["xenofilx_vs_oracle_bisulfite_nm"] += 1
            if int(oracle["conventional_nm"]) != int(oracle["xenofilx_bisulfite_nm"]):
                difference_counter["conversion_semantic_difference"] += 1

        picard_report: dict[str, object] = {}
        for label, records in picard_records_by_label.items():
            picard = records.get(record["identity_key"])
            if picard is None:
                picard_report[label] = None
                categories.append(f"{label}:missing")
                continue
            observed_picard_counts[label] += 1
            picard_report[label] = picard
            if oracle is None or xenofilx is None:
                continue
            picard_nm = int(picard["stored_nm"])
            conventional_nm = int(oracle["conventional_nm"])
            bisulfite_nm = int(oracle["xenofilx_bisulfite_nm"])
            delta_counts[label][f"vs_oracle_conventional:{picard_nm - conventional_nm}"] += 1
            delta_counts[label][f"vs_oracle_bisulfite:{picard_nm - bisulfite_nm}"] += 1
            if picard_nm == conventional_nm:
                conventional_match_counts[label] += 1
            else:
                categories.append(f"{label}:vs_oracle_conventional_nm")
            if picard_nm == bisulfite_nm:
                bisulfite_match_counts[label] += 1
            else:
                categories.append(f"{label}:vs_oracle_bisulfite_nm")
            if picard_nm == int(xenofilx["nm"]):
                difference_counter[f"{label}_equals_xenofilx_nm"] += 1
            else:
                categories.append(f"{label}:vs_xenofilx_nm")

        if categories:
            unique_categories = sorted(set(categories))
            difference_record = {
                "record_ordinal": record["record_ordinal"],
                "categories": unique_categories,
                "original": record,
                "picard": picard_report,
                "oracle": oracle,
                "xenofilx": xenofilx,
            }
            first_difference = add_difference(
                difference_counter,
                differences_file,
                difference_record,
                first_difference,
            )

    picard_summary: dict[str, object] = {}
    for label in picard_records_by_label:
        observed_count = observed_picard_counts[label]
        expected_count = len(original_records)
        picard_summary[label] = {
            "expected_records": expected_count,
            "observed_records": observed_count,
            "missing_records": expected_count - observed_count,
            "identity_equals_original": observed_count == expected_count
            and not any(difference_counter[f"{label}:identity:{field_name}"] for field_name in IDENTITY_FIELDS),
            "conventional_nm_equal_count": conventional_match_counts[label],
            "bisulfite_nm_equal_count": bisulfite_match_counts[label],
            "delta_counts": dict(sorted(delta_counts[label].items())),
        }

    return {
        "records_compared": len(original_records),
        "audit_oracle_records": len(oracle_rows),
        "audit_xenofilx_records": len(xenofilx_rows),
        "xenofilx_mode": xenofilx_mode,
        "xenofilx_expected_oracle_field": expected_xenofilx_oracle_field,
        "xenofilx_nm_equals_oracle_selected": not difference_counter["xenofilx_vs_oracle_selected_nm"],
        "xenofilx_nm_equals_oracle_conventional": not difference_counter["xenofilx_vs_oracle_conventional_nm"],
        "xenofilx_nm_equals_oracle_bisulfite": not difference_counter["xenofilx_vs_oracle_bisulfite_nm"],
        "difference_counts": dict(sorted(difference_counter.items())),
        "picard": picard_summary,
        "first_difference": first_difference,
    }


def main() -> int:
    arguments = parse_arguments()
    if arguments.records_read <= 0:
        raise ValueError("--records-read must be positive")
    picard_paths = dict(arguments.picard_bam)
    if len(picard_paths) != len(arguments.picard_bam):
        raise ValueError("Picard labels must be unique")
    original_records = read_original_mapped_records(
        arguments.samtools,
        arguments.original_bam,
        arguments.records_read,
    )
    universe = {record["identity_key"]: record for record in original_records}
    picard_records_by_label = {
        label: read_picard_nm_for_universe(arguments.samtools, path, universe)
        for label, path in picard_paths.items()
    }
    oracle_rows = read_audit_rows(arguments.oracle)
    xenofilx_rows = read_audit_rows(arguments.xenofilx)
    arguments.differences.parent.mkdir(parents=True, exist_ok=True)
    with arguments.differences.open("w", encoding="utf-8") as differences_file:
        comparison = compare_records(
            original_records,
            picard_records_by_label,
            oracle_rows,
            xenofilx_rows,
            arguments.xenofilx_mode,
            differences_file,
        )
    report = {
        "schema_version": "gate6.nm-read-comparison/v3",
        "original_bam": str(arguments.original_bam),
        "picard_bams": {label: str(path) for label, path in picard_paths.items()},
        "oracle": str(arguments.oracle),
        "xenofilx": str(arguments.xenofilx),
        "records_read": arguments.records_read,
        **comparison,
    }
    arguments.report.parent.mkdir(parents=True, exist_ok=True)
    arguments.report.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
