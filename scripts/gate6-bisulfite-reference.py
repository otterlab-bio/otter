#!/usr/bin/env python3
"""Create a strand-specific converted FASTA reference for Picard NM checks."""

from __future__ import annotations

import argparse
from pathlib import Path


def parse_arguments() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--input", required=True, type=Path)
    parser.add_argument("--output", required=True, type=Path)
    parser.add_argument("--conversion", choices=("ct", "ga"), required=True)
    return parser.parse_args()


def convert_reference(input_path: Path, output_path: Path, conversion: str) -> None:
    source_base, target_base = ("C", "T") if conversion == "ct" else ("G", "A")
    output_path.parent.mkdir(parents=True, exist_ok=True)
    with input_path.open("r", encoding="ascii", newline="") as input_file, output_path.open(
        "w", encoding="ascii", newline=""
    ) as output_file:
        for line in input_file:
            if line.startswith(">"):
                output_file.write(line)
            else:
                output_file.write(line.upper().replace(source_base, target_base))


def main() -> int:
    arguments = parse_arguments()
    convert_reference(arguments.input, arguments.output, arguments.conversion)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
