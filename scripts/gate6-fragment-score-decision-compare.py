#!/usr/bin/env python3
"""Compare modern bisulfite and legacy Picard score decisions on fixed fragments."""

from __future__ import annotations

import argparse
import csv
import json
from collections import Counter, defaultdict
from pathlib import Path


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--modern-graft", required=True, type=Path)
    parser.add_argument("--modern-host", required=True, type=Path)
    parser.add_argument("--legacy-graft-sam", required=True, type=Path)
    parser.add_argument("--legacy-host-sam", required=True, type=Path)
    parser.add_argument("--samples", required=True, type=Path)
    parser.add_argument("--names", required=True, type=Path)
    parser.add_argument("--report", required=True, type=Path)
    parser.add_argument("--threshold", default=6, type=int)
    parser.add_argument("--unmapped-penalty", default=8, type=int)
    return parser.parse_args()


def is_primary_mapped(flag: int) -> bool:
    return not flag & (0x4 | 0x100 | 0x800)


def mate_label(flag: int) -> str | None:
    is_first_mate = bool(flag & 0x40)
    is_second_mate = bool(flag & 0x80)
    if is_first_mate == is_second_mate:
        return None
    return "first" if is_first_mate else "second"


def load_sample_ids(samples_path: Path) -> set[str]:
    with samples_path.open("r", encoding="utf-8", newline="") as samples_file:
        reader = csv.DictReader(samples_file, delimiter="\t")
        if reader.fieldnames is None or "fragment_id" not in reader.fieldnames:
            raise ValueError(f"samples file has no fragment_id field: {samples_path}")
        return {row["fragment_id"] for row in reader}


def write_names_allowlist(names_path: Path, selected_fragments: set[str]) -> None:
    names_path.parent.mkdir(parents=True, exist_ok=True)
    names_path.write_text("\n".join(sorted(selected_fragments)) + "\n", encoding="utf-8")


def read_score_audit(path: Path, selected_fragments: set[str]) -> dict[str, dict[str, int]]:
    scores_by_fragment: dict[str, dict[str, int]] = defaultdict(dict)
    with path.open("r", encoding="utf-8", newline="") as audit_file:
        reader = csv.DictReader(audit_file, delimiter="\t")
        required_fields = {"qname", "flag", "classification_score"}
        if reader.fieldnames is None or not required_fields.issubset(reader.fieldnames):
            raise ValueError(f"unexpected score audit header: {reader.fieldnames}")
        for row in reader:
            fragment_id = row["qname"]
            if fragment_id not in selected_fragments:
                continue
            flag = int(row["flag"])
            if not is_primary_mapped(flag):
                continue
            mate = mate_label(flag)
            if mate is None or mate in scores_by_fragment[fragment_id]:
                raise ValueError(f"ambiguous modern primary record for {fragment_id}")
            scores_by_fragment[fragment_id][mate] = int(row["classification_score"])
    return scores_by_fragment


def parse_cigar_score(cigar: str, nm: int) -> int:
    index = 0
    insertion_bases = 0
    soft_clip_bases = 0
    while index < len(cigar):
        digit_start = index
        while index < len(cigar) and cigar[index].isdigit():
            index += 1
        if digit_start == index or index >= len(cigar):
            raise ValueError(f"invalid CIGAR: {cigar!r}")
        length = int(cigar[digit_start:index])
        operation = cigar[index]
        index += 1
        if operation == "I":
            insertion_bases += length
        elif operation == "S":
            soft_clip_bases += length
    return nm + insertion_bases + soft_clip_bases


def extract_nm(auxiliary_fields: list[str]) -> int:
    for field in auxiliary_fields:
        if field.startswith("NM:i:"):
            return int(field[5:])
    raise ValueError("missing NM:i tag")


def read_legacy_sam(path: Path, selected_fragments: set[str]) -> dict[str, dict[str, int]]:
    scores_by_fragment: dict[str, dict[str, int]] = defaultdict(dict)
    with path.open("r", encoding="utf-8", newline="") as sam_file:
        for line in sam_file:
            if line.startswith("@"):
                continue
            fields = line.rstrip("\n").split("\t")
            if len(fields) < 11:
                raise ValueError(f"malformed SAM record in {path}: {line!r}")
            fragment_id = fields[0]
            if fragment_id not in selected_fragments:
                continue
            flag = int(fields[1])
            if not is_primary_mapped(flag):
                continue
            mate = mate_label(flag)
            if mate is None or mate in scores_by_fragment[fragment_id]:
                raise ValueError(f"ambiguous legacy primary record for {fragment_id}")
            scores_by_fragment[fragment_id][mate] = parse_cigar_score(
                fields[5], extract_nm(fields[11:])
            )
    return scores_by_fragment


def paired_scores(scores: dict[str, int], unmapped_penalty: int) -> tuple[int, int]:
    return scores.get("first", unmapped_penalty), scores.get("second", unmapped_penalty)


def modern_decision(
    graft_scores: dict[str, int],
    host_scores: dict[str, int],
    threshold: int,
    unmapped_penalty: int,
) -> str:
    graft_first, graft_second = paired_scores(graft_scores, unmapped_penalty)
    if not host_scores:
        if graft_first < threshold or graft_second < threshold:
            return "graft_host_absent_any_mate_below_threshold"
        return "discarded_host_absent_both_mates_at_or_above_threshold"
    if graft_first >= threshold or graft_second >= threshold:
        return "discarded_threshold"
    host_first, host_second = paired_scores(host_scores, unmapped_penalty)
    graft_total = graft_first + graft_second
    host_total = host_first + host_second
    if graft_total < host_total:
        return "graft_better_total"
    if host_total < graft_total:
        return "host_better_total"
    return "discarded_tie"


def legacy_decision(
    graft_scores: dict[str, int],
    host_scores: dict[str, int],
    threshold: int,
    unmapped_penalty: int,
) -> str:
    graft_first, graft_second = paired_scores(graft_scores, unmapped_penalty)
    if not host_scores:
        if graft_first < threshold or graft_second < threshold:
            return "graft_host_absent_any_mate_below_threshold"
        return "discarded_host_absent_both_mates_at_or_above_threshold"
    host_first, host_second = paired_scores(host_scores, unmapped_penalty)
    if graft_first >= threshold or graft_second >= threshold:
        return "discarded_threshold"
    if (graft_first + graft_second) < (host_first + host_second):
        return "graft_better_mean"
    if (host_first + host_second) < (graft_first + graft_second):
        return "host_better_mean"
    return "discarded_tie"


def main() -> int:
    arguments = parse_arguments()
    selected_fragments = load_sample_ids(arguments.samples)
    write_names_allowlist(arguments.names, selected_fragments)
    modern_graft = read_score_audit(arguments.modern_graft, selected_fragments)
    modern_host = read_score_audit(arguments.modern_host, selected_fragments)
    legacy_graft = read_legacy_sam(arguments.legacy_graft_sam, selected_fragments)
    legacy_host = read_legacy_sam(arguments.legacy_host_sam, selected_fragments)

    decision_pairs: Counter[str] = Counter()
    host_absent_score_transitions: Counter[str] = Counter()
    missing_score_records: Counter[str] = Counter()
    for fragment_id in selected_fragments:
        if fragment_id not in modern_graft:
            missing_score_records["modern_graft"] += 1
            continue
        if fragment_id not in legacy_graft:
            missing_score_records["legacy_graft"] += 1
            continue
        modern = modern_decision(
            modern_graft[fragment_id],
            modern_host.get(fragment_id, {}),
            arguments.threshold,
            arguments.unmapped_penalty,
        )
        legacy = legacy_decision(
            legacy_graft[fragment_id],
            legacy_host.get(fragment_id, {}),
            arguments.threshold,
            arguments.unmapped_penalty,
        )
        decision_pairs[f"modern={modern};legacy={legacy}"] += 1
        if not modern_host.get(fragment_id, {}) and not legacy_host.get(fragment_id, {}):
            modern_first, modern_second = paired_scores(
                modern_graft[fragment_id], arguments.unmapped_penalty
            )
            legacy_first, legacy_second = paired_scores(
                legacy_graft[fragment_id], arguments.unmapped_penalty
            )
            host_absent_score_transitions[
                "modern_scores="
                f"{modern_first},{modern_second};legacy_scores={legacy_first},{legacy_second}"
            ] += 1

    report = {
        "schema_version": "gate6.fragment-score-decision-comparison/v1",
        "sample_size": len(selected_fragments),
        "threshold": arguments.threshold,
        "unmapped_penalty": arguments.unmapped_penalty,
        "decision_pairs": dict(sorted(decision_pairs.items())),
        "host_absent_score_transitions": dict(
            sorted(host_absent_score_transitions.items(), key=lambda item: (-item[1], item[0]))
        ),
        "missing_score_records": dict(sorted(missing_score_records.items())),
    }
    arguments.report.parent.mkdir(parents=True, exist_ok=True)
    arguments.report.write_text(json.dumps(report, indent=2, sort_keys=True) + "\n", encoding="utf-8")

    return 0 if not missing_score_records else 1


if __name__ == "__main__":
    raise SystemExit(main())
