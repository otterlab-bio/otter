#!/usr/bin/env python3
"""Generate metadata from the actual, already verified release payload."""
import datetime
import hashlib
import json
import pathlib
import sys


def main():
    directory, tag, commit = sys.argv[1:]
    dist = pathlib.Path(directory)
    files = sorted(p for p in dist.iterdir() if p.is_file() and p.name not in {"manifest.json", "checksums.txt"})
    assets = []
    for path in files:
        if path.suffix == ".yaml":
            assets.append({"name": path.name, "kind": "env_yaml"})
        else:
            stem = path.name.removesuffix("-static")
            _, os_name, arch = stem.rsplit("-", 2)
            assets.append({"name": path.name, "os": os_name, "arch": arch,
                           "linkage": "static" if os_name == "linux" else "system"})
    manifest = {"tag": tag, "commit": commit,
                "generated_at_utc": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
                "artifacts": assets}
    (dist / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n", encoding="utf-8")
    with (dist / "checksums.txt").open("w", encoding="utf-8") as output:
        for path in sorted([*files, dist / "manifest.json"]):
            with path.open("rb") as stream:
                digest = hashlib.file_digest(stream, "sha256").hexdigest()
            output.write(f"{digest}  {path.name}\n")


if __name__ == "__main__":
    main()
