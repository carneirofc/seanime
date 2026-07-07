"""Unit tests for the pure helpers in extension_model (no running Qt app needed).

``ExtensionModel``/``ExtensionFilterProxy`` are QObjects that need a Qt
application to instantiate, so these tests target the module-level normalization
helpers ``type_label`` and ``_row_of`` and the row-building logic they feed.
"""

import os
import sys
import unittest

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from seanime_qt.extension_model import _row_of, type_label


class TypeLabelTests(unittest.TestCase):
    def test_known_types(self):
        self.assertEqual(type_label("anime-torrent-provider"), "Torrent provider")
        self.assertEqual(type_label("manga-provider"), "Manga provider")
        self.assertEqual(type_label("onlinestream-provider"), "Streaming provider")
        self.assertEqual(type_label("plugin"), "Plugin")
        self.assertEqual(type_label("custom-source"), "Custom source")

    def test_unknown_type_is_title_cased(self):
        self.assertEqual(type_label("some-new-kind"), "Some New Kind")

    def test_empty_falls_back(self):
        self.assertEqual(type_label(""), "Extension")
        self.assertEqual(type_label(None), "Extension")


class RowOfTests(unittest.TestCase):
    def test_maps_fields_and_derives_type_label(self):
        row = _row_of(
            {
                "id": "nyaa",
                "name": "Nyaa",
                "version": "1.2.3",
                "type": "anime-torrent-provider",
                "language": "go",
                "lang": "multi",
                "author": "Seanime",
                "description": "Search torrents",
                "manifestURI": "https://example.com/nyaa.json",
            }
        )
        self.assertEqual(row["extId"], "nyaa")
        self.assertEqual(row["name"], "Nyaa")
        self.assertEqual(row["version"], "1.2.3")
        self.assertEqual(row["typeLabel"], "Torrent provider")
        self.assertEqual(row["manifestUri"], "https://example.com/nyaa.json")
        self.assertFalse(row["isBuiltin"])
        self.assertFalse(row["disabled"])
        self.assertFalse(row["invalid"])
        self.assertFalse(row["installed"])

    def test_builtin_flag_from_manifest(self):
        row = _row_of({"id": "local", "name": "Local", "manifestURI": "builtin"})
        self.assertTrue(row["isBuiltin"])

    def test_name_falls_back_to_id(self):
        row = _row_of({"id": "abc"})
        self.assertEqual(row["name"], "abc")

    def test_flags_are_passed_through(self):
        disabled = _row_of({"id": "a"}, disabled=True)
        self.assertTrue(disabled["disabled"])

        invalid = _row_of({"id": "b"}, invalid=True, invalid_reason="bad manifest")
        self.assertTrue(invalid["invalid"])
        self.assertEqual(invalid["invalidReason"], "bad manifest")

        installed = _row_of({"id": "c"}, installed=True)
        self.assertTrue(installed["installed"])

    def test_missing_fields_default_to_empty(self):
        row = _row_of({"id": "x"})
        self.assertEqual(row["description"], "")
        self.assertEqual(row["author"], "")
        self.assertEqual(row["icon"], "")
        self.assertEqual(row["manifestUri"], "")

    def test_non_dict_input_is_tolerated(self):
        row = _row_of(None)
        self.assertEqual(row["extId"], "")
        self.assertEqual(row["name"], "")


if __name__ == "__main__":
    unittest.main()
