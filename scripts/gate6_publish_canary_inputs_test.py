import gzip
import hashlib
import importlib.util
import json
import tempfile
import unittest
from pathlib import Path


SCRIPT_PATH = Path(__file__).with_name("gate6_publish_canary_inputs.py")
SCRIPT_SPEC = importlib.util.spec_from_file_location("gate6_publish_canary_inputs", SCRIPT_PATH)
CANARY_PUBLISHER = importlib.util.module_from_spec(SCRIPT_SPEC)
assert SCRIPT_SPEC.loader is not None
SCRIPT_SPEC.loader.exec_module(CANARY_PUBLISHER)


class CanaryInputPublisherTest(unittest.TestCase):
    def test_publish_preserves_pairs_and_is_immutable(self):
        with tempfile.TemporaryDirectory() as temporary_directory:
            root = Path(temporary_directory)
            r1_path = root / "source_R1.fastq.gz"
            r2_path = root / "source_R2.fastq.gz"
            self.write_fastq(r1_path, ["fragment-a", "fragment-b", "fragment-c"], "1")
            self.write_fastq(r2_path, ["fragment-a", "fragment-b", "fragment-c"], "2")

            config = {
                "scenario": "rrbs",
                "accession": "SRR12345678",
                "source": {
                    "r1": {"path": str(r1_path), "sha256": f"sha256:{self.sha256(r1_path)}"},
                    "r2": {"path": str(r2_path), "sha256": f"sha256:{self.sha256(r2_path)}"},
                },
                "selection": {"seed": "gate6-test-seed-01", "modulus": 1, "remainder": 0},
                "reference": {"primary_id": "hg38", "release": "test", "manifest_sha256": "sha256:" + "a" * 64},
                "provenance": {"workflow_digest": "sha256:" + "b" * 64},
            }
            config_path = root / "config.json"
            config_path.write_text(json.dumps(config), encoding="utf-8")
            output_path = root / "published"

            CANARY_PUBLISHER.publish(config_path, output_path)
            manifest = json.loads((output_path / "canary-inputs.json").read_text(encoding="utf-8"))
            self.assertEqual(manifest["schema_version"], "otter.canary-inputs/v1")
            self.assertEqual(manifest["selection"]["paired_record_count"], 3)
            self.assertEqual(manifest["output"]["r1"]["path"], str(output_path / "R1.fastq.gz"))
            self.assertEqual(manifest["output"]["r2"]["path"], str(output_path / "R2.fastq.gz"))
            self.assertEqual(self.read_identifiers(output_path / "R1.fastq.gz"), ["fragment-a", "fragment-b", "fragment-c"])
            self.assertEqual(self.read_identifiers(output_path / "R2.fastq.gz"), ["fragment-a", "fragment-b", "fragment-c"])
            with self.assertRaises(FileExistsError):
                CANARY_PUBLISHER.publish(config_path, output_path)

    @staticmethod
    def write_fastq(path: Path, identifiers: list[str], mate: str):
        with path.open("wb") as raw_output:
            with gzip.GzipFile(fileobj=raw_output, mode="wb", filename="", mtime=0) as output_file:
                for identifier in identifiers:
                    output_file.write(f"@{identifier}/{mate}\nACGT\n+\n!!!!\n".encode("utf-8"))

    @staticmethod
    def read_identifiers(path: Path) -> list[str]:
        identifiers = []
        with gzip.open(path, "rt", encoding="utf-8") as input_file:
            while header := input_file.readline():
                identifiers.append(header.strip()[1:].removesuffix("/1").removesuffix("/2"))
                input_file.readline()
                input_file.readline()
                input_file.readline()
        return identifiers

    @staticmethod
    def sha256(path: Path) -> str:
        return hashlib.sha256(path.read_bytes()).hexdigest()


if __name__ == "__main__":
    unittest.main()
