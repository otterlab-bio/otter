#!/usr/bin/env python3
"""Publish deterministic paired FASTQ canary inputs with immutable provenance."""

from __future__ import annotations

import argparse
import gzip
import hashlib
import json
import os
import shutil
import sys
import tempfile
from datetime import UTC, datetime
from pathlib import Path
from typing import BinaryIO


CHUNK_SIZE = 1024 * 1024
SELECTION_ALGORITHM = "sha256-qname-modulus/v1"
MANIFEST_SCHEMA_VERSION = "otter.canary-inputs/v1"


def calculate_sha256(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as input_file:
        while chunk := input_file.read(CHUNK_SIZE):
            digest.update(chunk)
    return digest.hexdigest()


def describe_file(path: Path, recorded_path: Path | None = None) -> dict[str, object]:
    return {
        "path": str(recorded_path or path),
        "sha256": f"sha256:{calculate_sha256(path)}",
        "bytes": path.stat().st_size,
    }


def read_fastq_record(input_file: BinaryIO, source_path: Path) -> tuple[bytes, bytes, bytes, bytes] | None:
    header = input_file.readline()
    if not header:
        return None
    sequence = input_file.readline()
    separator = input_file.readline()
    quality = input_file.readline()
    if not sequence or not separator or not quality:
        raise ValueError(f"truncated FASTQ record in {source_path}")
    if not header.startswith(b"@") or not separator.startswith(b"+"):
        raise ValueError(f"invalid FASTQ record structure in {source_path}")
    return header, sequence, separator, quality


def fragment_identifier(header: bytes, source_path: Path) -> bytes:
    identifier = header[1:].split(maxsplit=1)[0].rstrip()
    if identifier.endswith(b"/1") or identifier.endswith(b"/2"):
        identifier = identifier[:-2]
    if not identifier:
        raise ValueError(f"empty FASTQ read identifier in {source_path}")
    return identifier


def select_fragment(identifier: bytes, seed: str, modulus: int, remainder: int) -> bool:
    digest = hashlib.sha256(seed.encode("utf-8") + b"\0" + identifier).digest()
    return int.from_bytes(digest[:8], byteorder="big") % modulus == remainder


def write_pair(
    r1_path: Path,
    r2_path: Path,
    output_r1_path: Path,
    output_r2_path: Path,
    seed: str,
    modulus: int,
    remainder: int,
) -> int:
    selected_pairs = 0
    with gzip.open(r1_path, "rb") as r1_input, gzip.open(r2_path, "rb") as r2_input:
        with output_r1_path.open("wb") as r1_raw, output_r2_path.open("wb") as r2_raw:
            with gzip.GzipFile(fileobj=r1_raw, mode="wb", filename="", mtime=0) as r1_output:
                with gzip.GzipFile(fileobj=r2_raw, mode="wb", filename="", mtime=0) as r2_output:
                    while True:
                        r1_record = read_fastq_record(r1_input, r1_path)
                        r2_record = read_fastq_record(r2_input, r2_path)
                        if r1_record is None and r2_record is None:
                            break
                        if r1_record is None or r2_record is None:
                            raise ValueError("paired source FASTQs contain different record counts")
                        r1_identifier = fragment_identifier(r1_record[0], r1_path)
                        r2_identifier = fragment_identifier(r2_record[0], r2_path)
                        if r1_identifier != r2_identifier:
                            raise ValueError(
                                f"paired FASTQ identifiers differ: {r1_identifier!r} != {r2_identifier!r}"
                            )
                        if select_fragment(r1_identifier, seed, modulus, remainder):
                            r1_output.write(b"".join(r1_record))
                            r2_output.write(b"".join(r2_record))
                            selected_pairs += 1
    return selected_pairs


def load_configuration(path: Path) -> dict[str, object]:
    with path.open("r", encoding="utf-8") as config_file:
        configuration = json.load(config_file)
    for required_field in ("scenario", "accession", "source", "selection", "reference", "provenance"):
        if required_field not in configuration:
            raise ValueError(f"configuration is missing required field {required_field!r}")
    return configuration


def resolve_source_paths(configuration: dict[str, object]) -> tuple[Path, Path]:
    source = configuration["source"]
    if not isinstance(source, dict):
        raise ValueError("source must be an object")
    r1_path = Path(str(source["r1"]["path"]))
    r2_path = Path(str(source["r2"]["path"]))
    if not r1_path.is_file() or not r2_path.is_file():
        raise ValueError("source FASTQ paths must be regular files")
    return r1_path, r2_path


def verify_source_checksums(configuration: dict[str, object], r1_path: Path, r2_path: Path) -> None:
    source = configuration["source"]
    assert isinstance(source, dict)
    for mate_name, source_path in (("r1", r1_path), ("r2", r2_path)):
        expected_checksum = str(source[mate_name]["sha256"])
        actual_checksum = f"sha256:{calculate_sha256(source_path)}"
        if actual_checksum != expected_checksum:
            raise ValueError(
                f"source {mate_name} checksum mismatch: expected {expected_checksum}, got {actual_checksum}"
            )


def publish(configuration_path: Path, output_directory: Path) -> Path:
    configuration = load_configuration(configuration_path)
    r1_path, r2_path = resolve_source_paths(configuration)
    verify_source_checksums(configuration, r1_path, r2_path)

    selection = configuration["selection"]
    if not isinstance(selection, dict):
        raise ValueError("selection must be an object")
    seed = str(selection["seed"])
    modulus = int(selection["modulus"])
    remainder = int(selection["remainder"])
    if len(seed) < 16 or modulus < 1 or remainder < 0 or remainder >= modulus:
        raise ValueError("selection seed, modulus, or remainder is invalid")

    output_parent = output_directory.parent
    output_parent.mkdir(parents=True, exist_ok=True)
    if output_directory.exists():
        raise FileExistsError(f"refusing to modify existing immutable output {output_directory}")

    staging_directory = Path(tempfile.mkdtemp(prefix=f".{output_directory.name}.staging-", dir=output_parent))
    try:
        output_r1_path = staging_directory / "R1.fastq.gz"
        output_r2_path = staging_directory / "R2.fastq.gz"
        selected_pairs = write_pair(r1_path, r2_path, output_r1_path, output_r2_path, seed, modulus, remainder)
        if selected_pairs == 0:
            raise ValueError("deterministic selection retained no paired records")

        output = {
            "r1": describe_file(output_r1_path, output_directory / "R1.fastq.gz"),
            "r2": describe_file(output_r2_path, output_directory / "R2.fastq.gz"),
        }
        configuration["schema_version"] = MANIFEST_SCHEMA_VERSION
        configuration["selection"] = {
            "algorithm": SELECTION_ALGORITHM,
            "seed": seed,
            "modulus": modulus,
            "remainder": remainder,
            "paired_record_count": selected_pairs,
        }
        configuration["source"] = {"r1": describe_file(r1_path), "r2": describe_file(r2_path)}
        configuration["output"] = output
        provenance = configuration["provenance"]
        assert isinstance(provenance, dict)
        provenance["created_at"] = datetime.now(UTC).isoformat().replace("+00:00", "Z")
        provenance["generator"] = "scripts/gate6_publish_canary_inputs.py"

        manifest_path = staging_directory / "canary-inputs.json"
        manifest_path.write_text(json.dumps(configuration, indent=2, sort_keys=True) + "\n", encoding="utf-8")
        checksum_lines = [
            f"{calculate_sha256(output_r1_path)}  R1.fastq.gz",
            f"{calculate_sha256(output_r2_path)}  R2.fastq.gz",
            f"{calculate_sha256(manifest_path)}  canary-inputs.json",
        ]
        (staging_directory / "checksums.sha256").write_text("\n".join(checksum_lines) + "\n", encoding="utf-8")
        os.rename(staging_directory, output_directory)
        return output_directory
    except Exception:
        shutil.rmtree(staging_directory, ignore_errors=True)
        raise


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--config", required=True, type=Path, help="canary input configuration JSON")
    parser.add_argument("--output", required=True, type=Path, help="new immutable output directory")
    arguments = parser.parse_args()
    try:
        published_directory = publish(arguments.config, arguments.output)
    except Exception as error:
        print(f"canary input publication failed: {error}", file=sys.stderr)
        return 1
    print(published_directory)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
