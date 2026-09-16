#!/usr/bin/env python3
"""Audit paired-end classifier predicates against a fixed membership sample."""

from __future__ import annotations

import argparse
import csv
import json
from collections import Counter, defaultdict
from pathlib import Path


Classification = str
ScoresByMate = dict[str, int]


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--sample", required=True, type=Path)
    parser.add_argument("--modern-graft", required=True, type=Path)
    parser.add_argument("--modern-host", required=True, type=Path)
    parser.add_argument("--legacy-graft", required=True, type=Path)
    parser.add_argument("--legacy-host", required=True, type=Path)
    parser.add_argument("--report", required=True, type=Path)
    parser.add_argument("--threshold", type=int, default=6)
    parser.add_argument("--unmapped-penalty", type=int, default=8)
    return parser.parse_args()


def is_primary_mapped(flag: int) -> bool:
    return not flag & (0x4 | 0x100 | 0x800)


def mate_label(flag: int) -> str | None:
    is_first_mate = bool(flag & 0x40)
    is_second_mate = bool(flag & 0x80)
    if is_first_mate == is_second_mate:
        return None
    return "first" if is_first_mate else "second"


def load_expected_memberships(sample_path: Path) -> dict[str, tuple[Classification, Classification]]:
    memberships: dict[str, tuple[Classification, Classification]] = {}
    with sample_path.open(encoding="utf-8", newline="") as sample_file:
        reader = csv.DictReader(sample_file, delimiter="\t")
        required_columns = {"fragment_id", "modern_classification", "legacy_classification"}
        if reader.fieldnames is None or not required_columns.issubset(reader.fieldnames):
            raise ValueError(f"unexpected sample header: {reader.fieldnames}")
        for row in reader:
            memberships[row["fragment_id"]] = (
                row["modern_classification"],
                row["legacy_classification"],
            )
    if not memberships:
        raise ValueError("membership sample is empty")
    return memberships


def read_modern_scores(scores_path: Path, selected_names: set[str]) -> dict[str, ScoresByMate]:
    scores_by_name: dict[str, ScoresByMate] = defaultdict(dict)
    with scores_path.open(encoding="utf-8", newline="") as scores_file:
        reader = csv.DictReader(scores_file, delimiter="\t")
        required_columns = {"qname", "flag", "classification_score"}
        if reader.fieldnames is None or not required_columns.issubset(reader.fieldnames):
            raise ValueError(f"unexpected score header: {reader.fieldnames}")
        for row in reader:
            if row["qname"] not in selected_names:
                continue
            flag = int(row["flag"])
            if not is_primary_mapped(flag):
                continue
            mate = mate_label(flag)
            if mate is None:
                raise ValueError(f"invalid paired-end flags for {row['qname']}")
            if mate in scores_by_name[row["qname"]]:
                raise ValueError(f"duplicate modern primary {mate} mate for {row['qname']}")
            scores_by_name[row["qname"]][mate] = int(row["classification_score"])
    return scores_by_name


def parse_cigar_score(cigar: str, nm: int) -> int:
    insertion_bases = 0
    soft_clip_bases = 0
    cursor = 0
    while cursor < len(cigar):
        length_start = cursor
        while cursor < len(cigar) and cigar[cursor].isdigit():
            cursor += 1
        if length_start == cursor or cursor >= len(cigar):
            raise ValueError(f"invalid CIGAR: {cigar!r}")
        operation_length = int(cigar[length_start:cursor])
        operation = cigar[cursor]
        cursor += 1
        if operation == "I":
            insertion_bases += operation_length
        elif operation == "S":
            soft_clip_bases += operation_length
    return nm + insertion_bases + soft_clip_bases


def extract_nm(optional_fields: list[str]) -> int:
    for field in optional_fields:
        if field.startswith("NM:i:"):
            return int(field[5:])
    raise ValueError("SAM record has no NM:i tag")


def read_legacy_scores(sam_path: Path, selected_names: set[str]) -> dict[str, ScoresByMate]:
    scores_by_name: dict[str, ScoresByMate] = defaultdict(dict)
    with sam_path.open(encoding="utf-8", newline="") as sam_file:
        for line in sam_file:
            fields = line.rstrip("\n").split("\t")
            if len(fields) < 11 or fields[0] not in selected_names:
                continue
            flag = int(fields[1])
            if not is_primary_mapped(flag):
                continue
            mate = mate_label(flag)
            if mate is None:
                raise ValueError(f"invalid paired-end flags for {fields[0]}")
            if mate in scores_by_name[fields[0]]:
                raise ValueError(f"duplicate legacy primary {mate} mate for {fields[0]}")
            scores_by_name[fields[0]][mate] = parse_cigar_score(fields[5], extract_nm(fields[11:]))
    return scores_by_name


def pair_scores(scores_by_mate: ScoresByMate, unmapped_penalty: int) -> tuple[int, int]:
    return scores_by_mate.get("first", unmapped_penalty), scores_by_mate.get("second", unmapped_penalty)


def classify_current_xenofilx(
    graft_scores: ScoresByMate,
    host_scores: ScoresByMate,
    threshold: int,
    unmapped_penalty: int,
) -> Classification:
    if not graft_scores:
        return "host" if host_scores else "discarded"

    graft_first, graft_second = pair_scores(graft_scores, unmapped_penalty)
    if not host_scores:
        if graft_first < threshold or graft_second < threshold:
            return "graft"
        return "discarded"

    if graft_first >= threshold or graft_second >= threshold:
        return "discarded"

    host_first, host_second = pair_scores(host_scores, unmapped_penalty)
    graft_total = graft_first + graft_second
    host_total = host_first + host_second
    if graft_total < host_total:
        return "graft"
    if host_total < graft_total:
        return "host"
    return "discarded"


def classify_any_low_mate(
    graft_scores: ScoresByMate,
    host_scores: ScoresByMate,
    threshold: int,
    unmapped_penalty: int,
) -> Classification:
    graft_first, graft_second = pair_scores(graft_scores, unmapped_penalty)
    if not host_scores:
        return "graft" if graft_first < threshold or graft_second < threshold else "discarded"
    if graft_first >= threshold or graft_second >= threshold:
        return "discarded"
    host_first, host_second = pair_scores(host_scores, unmapped_penalty)
    graft_total = graft_first + graft_second
    host_total = host_first + host_second
    if graft_total < host_total:
        return "graft"
    if host_total < graft_total:
        return "host"
    return "discarded"


def calculate_accuracy(predictions: dict[str, Classification], expected: dict[str, Classification]) -> dict[str, int]:
    correct = sum(predictions[name] == expected[name] for name in expected)
    return {"correct": correct, "incorrect": len(expected) - correct, "total": len(expected)}


def main() -> int:
    arguments = parse_arguments()
    expected_memberships = load_expected_memberships(arguments.sample)
    selected_names = set(expected_memberships)
    expected_modern = {name: membership[0] for name, membership in expected_memberships.items()}
    expected_legacy = {name: membership[1] for name, membership in expected_memberships.items()}

    modern_graft_scores = read_modern_scores(arguments.modern_graft, selected_names)
    modern_host_scores = read_modern_scores(arguments.modern_host, selected_names)
    legacy_graft_scores = read_legacy_scores(arguments.legacy_graft, selected_names)
    legacy_host_scores = read_legacy_scores(arguments.legacy_host, selected_names)

    missing_primary_scores: Counter[str] = Counter()
    score_deltas: Counter[str] = Counter()
    host_mapping_states: Counter[str] = Counter()
    score_shapes: Counter[str] = Counter()
    modern_predictions: dict[str, Classification] = {}
    legacy_predictions: dict[str, Classification] = {}

    for name in selected_names:
        if name not in modern_graft_scores:
            missing_primary_scores["modern_graft"] += 1
            continue
        if name not in legacy_graft_scores:
            missing_primary_scores["legacy_graft"] += 1
            continue
        modern_graft_pair = pair_scores(modern_graft_scores[name], arguments.unmapped_penalty)
        legacy_graft_pair = pair_scores(legacy_graft_scores[name], arguments.unmapped_penalty)
        if modern_graft_pair != legacy_graft_pair:
            score_deltas[
                f"modern={modern_graft_pair[0]},{modern_graft_pair[1]};"
                f"legacy={legacy_graft_pair[0]},{legacy_graft_pair[1]}"
            ] += 1
        modern_has_host = bool(modern_host_scores.get(name))
        legacy_has_host = bool(legacy_host_scores.get(name))
        host_mapping_states[f"modern_host={modern_has_host};legacy_host={legacy_has_host}"] += 1
        below_threshold_mates = sum(score < arguments.threshold for score in legacy_graft_pair)
        score_shapes[f"legacy_mates_below_threshold={below_threshold_mates}"] += 1
        modern_predictions[name] = classify_current_xenofilx(
            modern_graft_scores[name],
            modern_host_scores.get(name, {}),
            arguments.threshold,
            arguments.unmapped_penalty,
        )
        legacy_predictions[name] = classify_any_low_mate(
            legacy_graft_scores[name],
            legacy_host_scores.get(name, {}),
            arguments.threshold,
            arguments.unmapped_penalty,
        )

    report = {
        "schema_version": "gate6.paired-decision-audit/v2",
        "sample_size": len(selected_names),
        "threshold": arguments.threshold,
        "unmapped_penalty": arguments.unmapped_penalty,
        "missing_primary_scores": dict(sorted(missing_primary_scores.items())),
        "modern_xenofilx_replay_accuracy": calculate_accuracy(modern_predictions, expected_modern),
        "legacy_any_low_mate_replay_accuracy": calculate_accuracy(legacy_predictions, expected_legacy),
        "graft_score_delta_pairs": dict(sorted(score_deltas.items(), key=lambda item: (-item[1], item[0]))),
        "host_mapping_states": dict(sorted(host_mapping_states.items())),
        "legacy_score_shapes": dict(sorted(score_shapes.items())),
    }
    arguments.report.parent.mkdir(parents=True, exist_ok=True)
    arguments.report.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    expected_sample_size = len(selected_names)
    observed_sample_size = len(modern_predictions)
    return 0 if observed_sample_size == expected_sample_size else 1


if __name__ == "__main__":
    raise SystemExit(main())
