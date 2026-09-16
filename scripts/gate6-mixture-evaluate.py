#!/usr/bin/env python3
"""Evaluate graft-membership performance against a mixture truth manifest."""

from __future__ import annotations

import argparse
import csv
import json
import subprocess
from collections import Counter
from pathlib import Path


MetricValue = float | int | str


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--truth", required=True, type=Path)
    parser.add_argument(
        "--tool-output",
        action="append",
        required=True,
        metavar="NAME=BAM",
        help="filtered graft BAM to evaluate; may be repeated",
    )
    parser.add_argument("--samtools", required=True, type=Path)
    parser.add_argument("--report", required=True, type=Path)
    parser.add_argument(
        "--require-known-selection",
        action="store_true",
        help="fail after writing the report if a filtered BAM contains QNAMEs absent from truth",
    )
    parser.add_argument(
        "--graft-source",
        choices=("human", "mouse"),
        default="mouse",
        help="truth source treated as graft; the other source is treated as host",
    )
    return parser.parse_args()


def parse_tool_outputs(values: list[str]) -> dict[str, Path]:
    outputs: dict[str, Path] = {}
    for value in values:
        name, separator, path = value.partition("=")
        if not separator or not name or not path or name in outputs:
            raise ValueError(f"invalid or duplicate --tool-output: {value!r}")
        outputs[name] = Path(path)
    return outputs


def load_truth(truth_path: Path) -> dict[str, str]:
    truth_by_fragment: dict[str, str] = {}
    with truth_path.open(encoding="utf-8", newline="") as truth_file:
        reader = csv.DictReader(truth_file, delimiter="\t")
        required_columns = {"fragment_id", "known_source"}
        if reader.fieldnames is None or not required_columns.issubset(reader.fieldnames):
            raise ValueError(f"unexpected truth header: {reader.fieldnames}")
        for row in reader:
            fragment_id = row["fragment_id"]
            known_source = row["known_source"]
            if not fragment_id or known_source not in {"human", "mouse"}:
                raise ValueError(f"invalid truth row: {row}")
            if fragment_id in truth_by_fragment:
                raise ValueError(f"duplicate truth fragment: {fragment_id}")
            truth_by_fragment[fragment_id] = known_source
    if not truth_by_fragment:
        raise ValueError(f"truth manifest is empty: {truth_path}")
    return truth_by_fragment


def stream_selected_qnames(samtools_path: Path, bam_path: Path) -> tuple[set[str], int]:
    process = subprocess.Popen(
        [str(samtools_path), "view", str(bam_path)],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        encoding="utf-8",
    )
    if process.stdout is None:
        raise RuntimeError(f"could not read filtered BAM: {bam_path}")
    selected_qnames: set[str] = set()
    record_count = 0
    for line in process.stdout:
        fields = line.rstrip("\n").split("\t", 1)
        if not fields[0]:
            raise ValueError(f"empty QNAME in filtered BAM: {bam_path}")
        selected_qnames.add(fields[0])
        record_count += 1
    process.stdout.close()
    stderr = process.stderr.read() if process.stderr is not None else ""
    if process.wait() != 0:
        raise RuntimeError(f"samtools view failed for {bam_path}: {stderr.strip()}")
    return selected_qnames, record_count


def calculate_rate(numerator: int, denominator: int) -> float | None:
    if denominator == 0:
        return None
    return numerator / denominator


def evaluate_tool(
    truth_by_fragment: dict[str, str],
    selected_qnames: set[str],
    filtered_record_count: int,
    graft_source: str,
) -> dict[str, MetricValue | None | dict[str, int]]:
    host_source = "human" if graft_source == "mouse" else "mouse"
    truth_fragments = set(truth_by_fragment)
    unknown_selected = selected_qnames - truth_fragments
    confusion = Counter(
        f"true_{graft_source}_predicted_graft"
        if known_source == graft_source and fragment_id in selected_qnames
        else f"true_{graft_source}_predicted_non_graft"
        if known_source == graft_source
        else f"true_{host_source}_predicted_graft"
        if fragment_id in selected_qnames
        else f"true_{host_source}_predicted_non_graft"
        for fragment_id, known_source in truth_by_fragment.items()
    )
    true_positive = confusion[f"true_{graft_source}_predicted_graft"]
    false_negative = confusion[f"true_{graft_source}_predicted_non_graft"]
    false_positive = confusion[f"true_{host_source}_predicted_graft"]
    true_negative = confusion[f"true_{host_source}_predicted_non_graft"]
    positive_predictions = true_positive + false_positive
    actual_positive = true_positive + false_negative
    actual_negative = true_negative + false_positive
    accuracy = calculate_rate(true_positive + true_negative, len(truth_by_fragment))
    sensitivity = calculate_rate(true_positive, actual_positive)
    specificity = calculate_rate(true_negative, actual_negative)
    precision = calculate_rate(true_positive, positive_predictions)
    negative_predictive_value = calculate_rate(true_negative, true_negative + false_negative)
    false_positive_rate = calculate_rate(false_positive, actual_negative)
    false_negative_rate = calculate_rate(false_negative, actual_positive)
    balanced_accuracy = (
        None
        if sensitivity is None or specificity is None
        else (sensitivity + specificity) / 2
    )
    f1_score = (
        None
        if precision is None or sensitivity is None or precision + sensitivity == 0
        else 2 * precision * sensitivity / (precision + sensitivity)
    )
    return {
        "truth_fragments": len(truth_by_fragment),
        "filtered_records": filtered_record_count,
        "filtered_fragments": len(selected_qnames),
        "unknown_selected_fragments": len(unknown_selected),
        "confusion": dict(sorted(confusion.items())),
        "accuracy": accuracy,
        "sensitivity_graft_recall": sensitivity,
        "specificity_host_recall": specificity,
        "precision_graft_ppv": precision,
        "negative_predictive_value_host": negative_predictive_value,
        "false_positive_rate_host_as_graft": false_positive_rate,
        "false_negative_rate_graft_as_non_graft": false_negative_rate,
        "balanced_accuracy": balanced_accuracy,
        "f1_graft": f1_score,
    }


def main() -> int:
    arguments = parse_arguments()
    if not arguments.samtools.is_file():
        raise ValueError(f"samtools is not a file: {arguments.samtools}")
    if not arguments.truth.is_file():
        raise ValueError(f"truth manifest is not a file: {arguments.truth}")
    truth_by_fragment = load_truth(arguments.truth)
    tool_outputs = parse_tool_outputs(arguments.tool_output)
    report: dict[str, object] = {
        "schema_version": "gate6.mixture-graft-membership-evaluation/v2",
        "truth_path": str(arguments.truth.resolve()),
        "graft_source": arguments.graft_source,
        "host_source": "human" if arguments.graft_source == "mouse" else "mouse",
        "tools": {},
    }
    for tool_name, bam_path in tool_outputs.items():
        if not bam_path.is_file():
            raise ValueError(f"filtered BAM is not a file: {bam_path}")
        selected_qnames, filtered_record_count = stream_selected_qnames(arguments.samtools, bam_path)
        report["tools"][tool_name] = {
            "filtered_bam": str(bam_path.resolve()),
            **evaluate_tool(
                truth_by_fragment,
                selected_qnames,
                filtered_record_count,
                arguments.graft_source,
            ),
        }
    arguments.report.parent.mkdir(parents=True, exist_ok=True)
    arguments.report.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")
    print(json.dumps(report, indent=2, sort_keys=True))
    has_unknown_selected_fragments = any(
        tool_report["unknown_selected_fragments"] > 0
        for tool_report in report["tools"].values()
    )
    return 1 if arguments.require_known_selection and has_unknown_selected_fragments else 0


if __name__ == "__main__":
    raise SystemExit(main())
