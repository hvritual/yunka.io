#!/usr/bin/env python3
"""Negative controls for release identity and immutable artifact evidence."""
import copy
import json
from pathlib import Path
import tempfile
import unittest

import qualify_consumer_release as release


class ReleaseIdentityTests(unittest.TestCase):
    def setUp(self):
        self.lock = release.read_lock(release.ROOT / "tools/consumer-release-lock.json")

    def test_duplicate_module_cannot_hide_missing_module(self):
        value = copy.deepcopy(self.lock)
        value["modules"][3] = value["modules"][0]
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / "lock.json"
            path.write_text(json.dumps(value))
            with self.assertRaisesRegex(ValueError, "distinct"):
                release.read_lock(path)

    def test_changed_checksum_and_source_commit_are_rejected(self):
        row = self.lock["modules"][0]
        downloaded = {"Path": row["path"], "Version": row["version"], "Sum": row["sum"], "GoModSum": row["goModSum"], "Origin": {"Hash": self.lock["sdkSourceCommit"]}}
        for key in ("Path", "Version", "Sum", "GoModSum", "Origin"):
            with self.subTest(key=key):
                changed = copy.deepcopy(downloaded)
                changed[key] = {"Hash": "0" * 40} if key == "Origin" else "changed"
                with self.assertRaises(ValueError):
                    release.check_download(row, changed, self.lock["sdkSourceCommit"])

    def test_module_json_stream_handles_nested_objects_and_whitespace(self):
        self.assertEqual(release.json_stream(' {"Path":"a","Origin":{"Hash":"x"}}\n\n {"Path":"b"}\n'), [{"Path": "a", "Origin": {"Hash": "x"}}, {"Path": "b"}])
        with self.assertRaises(json.JSONDecodeError):
            release.json_stream('{"Path":"a"}\nnot-json')

    def test_archive_does_not_inherit_source_timestamps_or_modes(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bundle = root / "bundle"
            bundle.mkdir()
            (bundle / "yunka").write_bytes(b"same binary")
            release.pack(bundle, root / "a.tar.gz")
            (bundle / "yunka").chmod(0o777)
            release.pack(bundle, root / "b.tar.gz")
            self.assertEqual(release.sha(root / "a.tar.gz"), release.sha(root / "b.tar.gz"))


if __name__ == "__main__":
    unittest.main()
