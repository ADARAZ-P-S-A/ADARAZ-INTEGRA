// Minimal RAZ-RCP-1 punch receiver (Node.js 18+, no dependencies).
// Reader: Settings > Integrations > Synchronization, format http3,
// URL http://<this computer>:8080/odbicia, login rcp, password = RCP_HASLO.
const http = require("node:http");
const fs = require("node:fs");
const crypto = require("node:crypto");

const LOGIN = "rcp", HASLO = process.env.RCP_HASLO, FILE = "odbicia.tsv";
const seen = new Set();
const expected = Buffer.from("Basic " + Buffer.from(`${LOGIN}:${HASLO}`).toString("base64"));

function answer(res, code, body) {
  res.writeHead(code, { "Content-Type": "application/json" });
  res.end(body);
}

http.createServer((req, res) => {
  const got = Buffer.from(req.headers.authorization || "");
  if (got.length !== expected.length || !crypto.timingSafeEqual(got, expected)) {
    return answer(res, 401, '{"error":"unauthorized"}'); // punch stays in the reader
  }
  let raw = "";
  req.on("data", (chunk) => { if (raw.length < 4096) raw += chunk; });
  req.on("end", () => {
    const form = new URLSearchParams(raw);
    const rfid = form.get("rfidID") || "", machine = form.get("machineId") || "";
    const m = /^(\d{4})-(\d\d)-(\d\d) (\d\d):(\d\d):(\d\d)$/.exec(form.get("timestamp") || "");
    if (!/^[0-9a-f]{1,20}$/.test(rfid) || !m) {
      return answer(res, 400, '{"error":"bad request"}'); // dropped by the reader
    }
    const epoch = Date.UTC(m[1], m[2] - 1, m[3], m[4], m[5], m[6]) / 1000; // http3: UTC
    if (Number(m[1]) < 2026) return answer(res, 200, '{"stored":false,"reason":"clock"}');
    const key = `${epoch}\t${rfid}\t${machine}`;
    if (seen.has(key)) return answer(res, 200, '{"stored":false,"reason":"duplicate"}');
    const fd = fs.openSync(FILE, "a");                  // store first...
    fs.writeSync(fd, key.replace(/[\r\n]/g, " ") + "\n");
    fs.fsyncSync(fd);
    fs.closeSync(fd);
    seen.add(key);
    answer(res, 200, '{"stored":true}');                // ...then answer 200
  });
}).listen(8080);
