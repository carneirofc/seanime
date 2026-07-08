"""Unit tests for the pure helper in manga_search_model (no running Qt app)."""

import os
import sys
import unittest
from urllib.parse import parse_qs, urlparse

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "src"))

from seanime_qt.manga_search_model import proxied_image_url


class ProxiedImageUrlTests(unittest.TestCase):
    BASE = "http://127.0.0.1:43211"

    def test_empty_url_returns_empty(self):
        self.assertEqual(proxied_image_url(self.BASE, "", None), "")

    def test_wraps_url_through_image_proxy(self):
        out = proxied_image_url(self.BASE, "https://cdn.example/cover.jpg", None)
        parsed = urlparse(out)
        self.assertEqual(parsed.path, "/api/v1/image-proxy")
        self.assertEqual(
            parse_qs(parsed.query)["url"], ["https://cdn.example/cover.jpg"]
        )
        self.assertNotIn("headers", parse_qs(parsed.query))

    def test_includes_headers_when_present(self):
        out = proxied_image_url(
            self.BASE, "https://cdn.example/x.jpg", {"Referer": "https://site"}
        )
        headers = parse_qs(urlparse(out).query)["headers"][0]
        self.assertIn("Referer", headers)
        self.assertIn("https://site", headers)

    def test_strips_trailing_slash_on_base(self):
        out = proxied_image_url(self.BASE + "/", "https://cdn.example/x.jpg", None)
        self.assertEqual(urlparse(out).path, "/api/v1/image-proxy")
        self.assertNotIn("//api", out)


if __name__ == "__main__":
    unittest.main()
