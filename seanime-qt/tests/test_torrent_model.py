"""Unit tests for the pure helpers in torrent_model (no running Qt app needed)."""

import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from seanime_qt.torrent_model import format_torrent_date, human_size


class HumanSizeTests(unittest.TestCase):
    def test_bytes(self):
        self.assertEqual(human_size(512), "512 B")

    def test_kilobytes(self):
        self.assertEqual(human_size(2048), "2.0 KB")

    def test_gigabytes(self):
        self.assertEqual(human_size(1610612736), "1.5 GB")

    def test_zero_and_invalid(self):
        self.assertEqual(human_size(0), "")
        self.assertEqual(human_size(None), "")
        self.assertEqual(human_size("nope"), "")


class FormatTorrentDateTests(unittest.TestCase):
    def test_reduces_rfc3339_to_date(self):
        self.assertEqual(format_torrent_date("2023-01-02T15:04:05Z"), "2023-01-02")

    def test_passes_through_non_timestamp(self):
        self.assertEqual(format_torrent_date("yesterday"), "yesterday")

    def test_empty(self):
        self.assertEqual(format_torrent_date(None), "")


if __name__ == "__main__":
    unittest.main()
