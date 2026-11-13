"""Serves the site with caching off.

python3 -m http.server sends Last-Modified and no Cache-Control, so browsers
cache the ES modules heuristically and keep running stale code after an edit.
"""
import functools
import http.server
import pathlib
import sys

ROOT = pathlib.Path(__file__).resolve().parent.parent


class NoCache(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header("Cache-Control", "no-store")
        super().end_headers()


port = int(sys.argv[1]) if len(sys.argv) > 1 else 8000
handler = functools.partial(NoCache, directory=str(ROOT))
with http.server.ThreadingHTTPServer(("127.0.0.1", port), handler) as httpd:
    print(f"serving {ROOT} on http://localhost:{port}", flush=True)
    httpd.serve_forever()
