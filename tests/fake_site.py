"""A stand-in for MySetup's upload route, POST /api/scanner/reports, for tools/check-send.sh. It answers as the site does for a
report sent without a code (MySetup's docs/scanner/design.md, One tab; app 0.3.0): the first report it is sent gets 503 with the
site's own words, as when too many wait, and every later one 201 with a one-time link; a report sent with a code, or a request
not made as the app makes it, gets 400. It listens on this computer only, writes its port to <dir>/port, a line for each report
it is sent to <dir>/sends, and the last report it took to <dir>/received.
    python3 -B tests/fake_site.py <dir>
"""

import json
import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

TOKEN = "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789-_AbCdE"
BUSY = "Sending is busy right now. Try again in a few minutes, or copy the report and paste it in the box on your build page."


class Site(BaseHTTPRequestHandler):
    def do_POST(self):
        if (
            self.path != "/api/scanner/reports"
            or self.headers.get("Content-Type") != "application/json"
            or self.headers.get("Accept") != "application/json"
            or not self.headers.get("User-Agent", "").startswith("MySetup-Scanner/")
        ):
            self.answer(400, {"message": "Not sent as the app sends."})
            return
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        if "code" in body or not isinstance(body.get("report"), str):
            self.answer(400, {"message": "Not sent as the app sends since 0.3.0."})
            return
        sends = os.path.join(sys.argv[1], "sends")
        with open(sends, "a", encoding="utf-8", newline="") as f:
            f.write("sent\n")
        with open(sends, encoding="utf-8") as f:
            first = len(f.readlines()) == 1
        if first:
            self.answer(503, {"message": BUSY})
            return
        with open(os.path.join(sys.argv[1], "received"), "w", encoding="utf-8", newline="") as received:
            received.write(body["report"])
        self.answer(201, {"claim_url": "http://127.0.0.1:%d/scan/claim/%s" % (self.server.server_address[1], TOKEN),
                          "expires_at": "2026-11-24T23:30:00+01:00"})

    def answer(self, status, body):
        data = json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *args):
        pass


server = ThreadingHTTPServer(("127.0.0.1", 0), Site)
port = os.path.join(sys.argv[1], "port")
with open(port + ".new", "w", encoding="utf-8") as f:
    f.write(str(server.server_address[1]))
os.replace(port + ".new", port)
server.serve_forever()
