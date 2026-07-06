"""Unit tests for the pure torrent-destination helpers (no Qt required)."""

import os
import sys
import unittest

# Make ``seanime_qt`` importable when running from the repo root.
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from seanime_qt.destination import default_destination, sanitize_directory_name


class SanitizeDirectoryNameTests(unittest.TestCase):
    def test_replaces_disallowed_characters_with_spaces(self):
        self.assertEqual(sanitize_directory_name("Re:Zero / Season 2?"), "Re Zero Season 2")

    def test_collapses_and_trims_whitespace(self):
        self.assertEqual(sanitize_directory_name("  A   B  "), "A B")

    def test_strips_leading_and_trailing_dots(self):
        self.assertEqual(sanitize_directory_name("...Naruto..."), "Naruto")

    def test_empty_falls_back_to_untitled(self):
        self.assertEqual(sanitize_directory_name(""), "Untitled")
        self.assertEqual(sanitize_directory_name("///"), "Untitled")


class DefaultDestinationTests(unittest.TestCase):
    def test_uses_dirname_of_last_local_file(self):
        files = [
            {"path": "/library/Naruto/ep1.mkv"},
            {"path": "/library/Naruto/ep2.mkv"},
        ]
        self.assertEqual(default_destination(files, "/library", "Naruto"), "/library/Naruto")

    def test_normalizes_windows_separators(self):
        files = [{"path": r"D:\Anime\Bleach\ep1.mkv"}]
        self.assertEqual(default_destination(files, "D:/Anime", "Bleach"), "D:/Anime/Bleach")

    def test_joins_library_path_and_sanitized_title_when_no_files(self):
        self.assertEqual(default_destination([], "/library", "Re:Zero"), "/library/Re Zero")

    def test_empty_when_no_files_and_no_library_path(self):
        self.assertEqual(default_destination([], "", "Whatever"), "")


if __name__ == "__main__":
    unittest.main()
