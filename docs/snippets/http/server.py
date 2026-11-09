#!/usr/bin/env python3
# Minimal RAZ-RCP-1 punch receiver (Python 3.9+, standard library only).
# Reader: Settings > Integrations > Synchronization, format http3,
# URL http://<this computer>:8080/odbicia, login rcp, password = RCP_HASLO.
import base64, hmac, os, re
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Lock
from urllib.parse import parse_qs

LOGIN, HASLO = "rcp", os.environ["RCP_HASLO"]
FILE, lock, seen = "odbicia.tsv", Lock(), set()

class Handler(BaseHTTPRequestHandler):
    def answer(self, code, body):
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body.encode())

    def do_POST(self):
        auth = "Basic " + base64.b64encode(f"{LOGIN}:{HASLO}".encode()).decode()
        if not hmac.compare_digest(self.headers.get("Authorization", ""), auth):
            return self.answer(401, '{"error":"unauthorized"}')    # punch stays in the reader
        n = int(self.headers.get("Content-Length") or 0)
        if n > 4096:
            self.close_connection = True
            return self.answer(400, '{"error":"too large"}')
        form = parse_qs(self.rfile.read(n).decode("utf-8", "replace"))
        rfid = form.get("rfidID", [""])[0]
        machine = re.sub(r"[\t\r\n]", " ", form.get("machineId", [""])[0])
        try:
            ts = datetime.strptime(form.get("timestamp", [""])[0], "%Y-%m-%d %H:%M:%S")
        except ValueError:
            ts = None
        if not re.fullmatch(r"[0-9a-f]{1,20}", rfid) or ts is None:
            return self.answer(400, '{"error":"bad request"}')     # dropped by the reader
        epoch = int(ts.replace(tzinfo=timezone.utc).timestamp())  # http3: UTC
        if ts.year < 2026:
            return self.answer(200, '{"stored":false,"reason":"clock"}')
        key = (epoch, rfid, machine)
        with lock:
            if key in seen:
                return self.answer(200, '{"stored":false,"reason":"duplicate"}')
            with open(FILE, "a", encoding="utf-8") as f:            # store first...
                f.write(f"{epoch}\t{rfid}\t{machine}\n")
                f.flush()
                os.fsync(f.fileno())
            seen.add(key)
        self.answer(200, '{"stored":true}')                         # ...then answer 200

ThreadingHTTPServer(("0.0.0.0", 8080), Handler).serve_forever()
