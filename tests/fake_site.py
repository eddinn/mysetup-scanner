"""A stand-in for MySetup's upload route, POST /api/scanner/reports, for tools/check-send.sh. It answers as the site does
(MySetup's docs/scanner/design.md, Stage 2): 201 with the review's address for the one right code, typed in any case and with
any spaces and dashes as the site takes it, 422 with the site's own words for any other, 400 for a request not made as the app makes it. It listens on this computer only, writes its port to <dir>/port,
each code it is sent to <dir>/codes, and the report sent with the right code to <dir>/received.
    python3 -B tests/fake_site.py <dir>
"""

import json
import os
import sys
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

CODE = "K7QD-M2XP"
WRONG = "This code is wrong, was already used or has expired. Get a new code on your build page and type it again."


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
        with open(os.path.join(sys.argv[1], "codes"), "a", encoding="utf-8", newline="") as codes:
            codes.write(body["code"] + "\n")
        if body["code"].replace(" ", "").replace("-", "").upper() != CODE.replace("-", ""):
            self.answer(422, {"message": WRONG})
            return
        with open(os.path.join(sys.argv[1], "received"), "w", encoding="utf-8", newline="") as received:
            received.write(body["report"])
        self.answer(201, {"review_url": "http://127.0.0.1:%d/setups/01M7/components/scan/01M8" % self.server.server_address[1]})

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
