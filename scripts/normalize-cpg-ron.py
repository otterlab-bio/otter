#!/usr/bin/env python3
import re
import sys
from pathlib import Path

if len(sys.argv) != 3:
    raise SystemExit("usage: normalize-cpg-ron.py <input.ron> <output.tsv>")

input_path = Path(sys.argv[1])
output_path = Path(sys.argv[2])
record_pattern = re.compile(
    rb'chr: "([^"]+)",\s*start: (\d+),\s*end: (\d+),\s*strand: \'([^\']+)\''
)
input_bytes = input_path.read_bytes()
record_count = 0
with output_path.open("wb") as output_file:
    for match in record_pattern.finditer(input_bytes):
        output_file.write(
            match.group(1)
            + b"\t"
            + match.group(2)
            + b"\t"
            + match.group(3)
            + b"\t"
            + match.group(4)
            + b"\n"
        )
        record_count += 1
print(f"input_bytes={len(input_bytes)}")
print(f"record_count={record_count}")
