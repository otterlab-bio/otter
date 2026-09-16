#!/usr/bin/env bash
#
# Print selected fields from JSON evidence files as `key=value` lines, for use in job summaries.
#
# Workflows used to inline multi-line Python to read their own evidence back. That is fragile: the
# code sits inside a YAML block scalar, so any line that loses its indentation silently terminates
# the scalar and breaks the entire workflow. Keeping the logic in a real script removes that hazard
# and makes the extraction testable outside CI.
#
# usage:
#   summary-field.sh <path> [<path> ...]
#
# Each path is `<json-file>#<dotted.field.path>`, for example:
#   summary-field.sh ci-artifacts/gate-a/roundtrip.json#records_written
#   summary-field.sh ci-artifacts/evidence/membership.json#categories.output_membership
#
# A missing file, a missing key, or a key that holds a container prints `-`, because a briefing must
# still render when a job died before producing evidence. Output is always `key=value`, where `key`
# is the last path segment, so callers can read it with a simple prefix match.
set -euo pipefail

if [[ ${#} -lt 1 ]]; then
  echo "usage: summary-field.sh <json-file>#<dotted.field.path> [...]" >&2
  exit 2
fi

# NUL-separate the arguments so paths containing spaces survive the hop into Python.
printf '%s\0' "$@" | python3 -c '
import json
import pathlib
import sys

# Read the NUL-separated path specifications from stdin.
raw = sys.stdin.buffer.read()
specifications = [item.decode("utf-8") for item in raw.split(b"\0") if item]

for specification in specifications:
    if "#" not in specification:
        print(f"{specification}=-")
        continue

    file_part, _, dotted_path = specification.partition("#")
    leaf = dotted_path.rsplit(".", 1)[-1]

    document = None
    path = pathlib.Path(file_part)
    if path.is_file():
        try:
            document = json.loads(path.read_text())
        except (json.JSONDecodeError, OSError):
            document = None

    value = document
    for segment in dotted_path.split("."):
        if isinstance(value, dict) and segment in value:
            value = value[segment]
        else:
            value = None
            break

    # A container value is not useful in a table cell; report it as absent.
    if isinstance(value, (dict, list)) or value is None:
        print(f"{leaf}=-")
    else:
        print(f"{leaf}={value}")
'
