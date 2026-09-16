#!/usr/bin/env python3
"""Create a deterministic paired-end human/mouse mixture with hidden truth labels."""

from __future__ import annotations

import argparse
import csv
import gzip
import hashlib
import heapq
import json
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Iterator, TextIO


@dataclass(frozen=True)
class FastqPair:
    fragment_id: str
    read_one: tuple[str, str, str, str]
    read_two: tuple[str, str, str, str]


@dataclass
class SourceSummary:
    source: str
    available_fragments: int
    requested_fragments: int
    selected_fragments: int


class MixtureInputError(ValueError):
    """Raised when paired FASTQ input violates the benchmark contract."""


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--human-r1", required=True, type=Path)
    parser.add_argument("--human-r2", required=True, type=Path)
    parser.add_argument("--mouse-r1", required=True, type=Path)
    parser.add_argument("--mouse-r2", required=True, type=Path)
    parser.add_argument("--output-directory", required=True, type=Path)
    parser.add_argument("--human-fraction", required=True, type=float)
    parser.add_argument("--total-fragments", required=True, type=int)
    parser.add_argument("--seed", required=True)
    parser.add_argument("--replicate", required=True, type=int)
    return parser.parse_args()


def open_text(path: Path, mode: str) -> TextIO:
    if path.suffix == ".gz":
        return gzip.open(path, mode, encoding="utf-8", newline="")
    return path.open(mode, encoding="utf-8", newline="")


def validate_arguments(arguments: argparse.Namespace) -> None:
    if not 0.0 <= arguments.human_fraction <= 1.0:
        raise MixtureInputError("--human-fraction must be between 0 and 1")
    if arguments.total_fragments <= 0:
        raise MixtureInputError("--total-fragments must be positive")
    if arguments.replicate <= 0:
        raise MixtureInputError("--replicate must be positive")
    required_paths = (arguments.human_r1, arguments.human_r2, arguments.mouse_r1, arguments.mouse_r2)
    missing_paths = [str(path) for path in required_paths if not path.is_file()]
    if missing_paths:
        raise MixtureInputError(f"missing FASTQ files: {', '.join(missing_paths)}")


def read_fastq_records(handle: TextIO, path: Path) -> Iterator[tuple[str, str, str, str]]:
    while True:
        header = handle.readline()
        if not header:
            return
        sequence = handle.readline()
        plus = handle.readline()
        quality = handle.readline()
        if not sequence or not plus or not quality:
            raise MixtureInputError(f"truncated FASTQ record in {path}")
        if not header.startswith("@") or not plus.startswith("+"):
            raise MixtureInputError(f"malformed FASTQ record in {path}")
        sequence_text = sequence.rstrip("\n")
        quality_text = quality.rstrip("\n")
        if len(sequence_text) != len(quality_text):
            raise MixtureInputError(f"sequence and quality lengths differ in {path}")
        yield header.rstrip("\n"), sequence_text, plus.rstrip("\n"), quality_text


def normalize_fragment_id(header: str) -> str:
    first_token = header[1:].split()[0]
    if first_token.endswith("/1") or first_token.endswith("/2"):
        return first_token[:-2]
    return first_token


def iterate_pairs(read_one_path: Path, read_two_path: Path) -> Iterator[FastqPair]:
    with open_text(read_one_path, "rt") as read_one_file, open_text(read_two_path, "rt") as read_two_file:
        read_one_records = read_fastq_records(read_one_file, read_one_path)
        read_two_records = read_fastq_records(read_two_file, read_two_path)
        pair_index = 0
        while True:
            try:
                read_one = next(read_one_records)
            except StopIteration:
                try:
                    next(read_two_records)
                except StopIteration:
                    return
                raise MixtureInputError(f"R2 contains more records than R1 near pair {pair_index}")
            try:
                read_two = next(read_two_records)
            except StopIteration as error:
                raise MixtureInputError(f"R1 contains more records than R2 near pair {pair_index}") from error
            read_one_id = normalize_fragment_id(read_one[0])
            read_two_id = normalize_fragment_id(read_two[0])
            if read_one_id != read_two_id:
                raise MixtureInputError(
                    f"mate IDs differ at pair {pair_index}: {read_one_id!r} != {read_two_id!r}"
                )
            yield FastqPair(read_one_id, read_one, read_two)
            pair_index += 1


def count_pairs(read_one_path: Path, read_two_path: Path) -> int:
    return sum(1 for _ in iterate_pairs(read_one_path, read_two_path))


def selection_score(seed: str, source: str, fragment_id: str) -> int:
    digest = hashlib.blake2b(
        f"{seed}\0{source}\0{fragment_id}".encode("utf-8"), digest_size=8
    ).digest()
    return int.from_bytes(digest, byteorder="big", signed=False)


def select_by_hash(
    read_one_path: Path,
    read_two_path: Path,
    source: str,
    seed: str,
    requested_fragments: int,
    available_fragments: int,
) -> set[str]:
    if requested_fragments == 0:
        return set()
    if requested_fragments > available_fragments:
        raise MixtureInputError(
            f"requested {requested_fragments} {source} fragments, only {available_fragments} available"
        )
    selected_by_score: list[tuple[int, str]] = []
    for pair in iterate_pairs(read_one_path, read_two_path):
        score = selection_score(seed, source, pair.fragment_id)
        candidate = (-score, pair.fragment_id)
        if len(selected_by_score) < requested_fragments:
            heapq.heappush(selected_by_score, candidate)
        elif candidate > selected_by_score[0]:
            heapq.heapreplace(selected_by_score, candidate)
    return {fragment_id for _, fragment_id in selected_by_score}


def rewrite_header(source: str, fragment_id: str) -> str:
    return f"@mix_{source}__{fragment_id}\n"


def write_pair(
    read_one_file: TextIO,
    read_two_file: TextIO,
    pair: FastqPair,
    source: str,
    truth_file: csv.DictWriter,
) -> None:
    output_fragment_id = f"mix_{source}__{pair.fragment_id}"
    for output_file, record in ((read_one_file, pair.read_one), (read_two_file, pair.read_two)):
        output_file.write(rewrite_header(source, pair.fragment_id))
        output_file.write(f"{record[1]}\n{record[2]}\n{record[3]}\n")
    truth_file.writerow(
        {
            "fragment_id": output_fragment_id,
            "known_source": source,
            "original_fragment_id": pair.fragment_id,
            "read_pairs": 1,
        }
    )


def write_selected_pairs(
    read_one_path: Path,
    read_two_path: Path,
    source: str,
    selected_fragment_ids: set[str],
    read_one_file: TextIO,
    read_two_file: TextIO,
    truth_file: csv.DictWriter,
) -> int:
    selected_count = 0
    for pair in iterate_pairs(read_one_path, read_two_path):
        if pair.fragment_id not in selected_fragment_ids:
            continue
        write_pair(read_one_file, read_two_file, pair, source, truth_file)
        selected_count += 1
    if selected_count != len(selected_fragment_ids):
        raise MixtureInputError(
            f"selected {selected_count} of {len(selected_fragment_ids)} requested {source} fragments during output"
        )
    return selected_count


def write_mixture(
    output_directory: Path,
    source_inputs: dict[str, tuple[Path, Path, str, set[str]]],
) -> dict[str, int]:
    output_directory.mkdir(parents=True, exist_ok=False)
    truth_path = output_directory / "truth.tsv"
    selected_counts: dict[str, int] = {}
    with gzip.open(output_directory / "mixture_R1.fastq.gz", "wt", encoding="utf-8", newline="") as read_one_file, gzip.open(output_directory / "mixture_R2.fastq.gz", "wt", encoding="utf-8", newline="") as read_two_file, truth_path.open("w", encoding="utf-8", newline="") as truth_handle:
        truth_writer = csv.DictWriter(
            truth_handle,
            fieldnames=["fragment_id", "known_source", "original_fragment_id", "read_pairs"],
            delimiter="\t",
        )
        truth_writer.writeheader()
        for source in ("human", "mouse"):
            read_one_path, read_two_path, selection_seed, selected_fragment_ids = source_inputs[source]
            selected_counts[source] = write_selected_pairs(
                read_one_path,
                read_two_path,
                source,
                selected_fragment_ids,
                read_one_file,
                read_two_file,
                truth_writer,
            )
    return selected_counts


def main() -> int:
    arguments = parse_arguments()
    validate_arguments(arguments)
    human_requested = round(arguments.total_fragments * arguments.human_fraction)
    mouse_requested = arguments.total_fragments - human_requested
    summaries: list[SourceSummary] = []
    source_inputs: dict[str, tuple[Path, Path, str, set[str]]] = {}
    for source, (read_one_path, read_two_path, requested_fragments) in (
        ("human", (arguments.human_r1, arguments.human_r2, human_requested)),
        ("mouse", (arguments.mouse_r1, arguments.mouse_r2, mouse_requested)),
    ):
        available_fragments = count_pairs(read_one_path, read_two_path)
        selection_seed = f"{arguments.seed}\0replicate={arguments.replicate}"
        selected_fragment_ids = select_by_hash(
            read_one_path,
            read_two_path,
            source,
            selection_seed,
            requested_fragments,
            available_fragments,
        )
        source_inputs[source] = (read_one_path, read_two_path, selection_seed, selected_fragment_ids)
        summaries.append(SourceSummary(source, available_fragments, requested_fragments, len(selected_fragment_ids)))

    output_directory = arguments.output_directory.resolve()
    selected_counts = write_mixture(output_directory, source_inputs)
    summaries = [
        SourceSummary(
            summary.source,
            summary.available_fragments,
            summary.requested_fragments,
            selected_counts[summary.source],
        )
        for summary in summaries
    ]
    total_selected = sum(selected_counts.values())
    manifest = {
        "schema_version": "gate6.paired-fastq-mixture/v1",
        "seed": arguments.seed,
        "replicate": arguments.replicate,
        "human_fraction_requested": arguments.human_fraction,
        "total_fragments_requested": arguments.total_fragments,
        "total_fragments_selected": total_selected,
        "human_fraction_selected": selected_counts["human"] / total_selected,
        "source_summaries": [asdict(summary) for summary in summaries],
        "output_files": {
            "r1": str(output_directory / "mixture_R1.fastq.gz"),
            "r2": str(output_directory / "mixture_R2.fastq.gz"),
            "truth": str(output_directory / "truth.tsv"),
        },
    }
    (output_directory / "manifest.json").write_text(
        json.dumps(manifest, indent=2, sort_keys=True) + "\n", encoding="utf-8"
    )
    print(json.dumps(manifest, indent=2, sort_keys=True))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
