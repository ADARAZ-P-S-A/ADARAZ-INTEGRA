# Punch push (HTTP)

The simplest integration: every configured interval the reader sends new
punches to your server's URL, each in its own `POST` request. Your server
stores the punch and answers `200`. Nothing else is needed.

```
reader ── POST rfidID, timestamp, machineId ──▶ your server
       ◀────────── 200 {"stored":true} ─────────
```

## 1. The server

A complete server in four languages: it checks the login and password, writes
the punch to a file and only then answers `200`, and recognises duplicates and
punches made with an unset clock. Each is a few dozen lines with no libraries.

::: code-group

<<< @/../examples/RAZ-RCP-1/http/minimal/server.py [Python]

<<< @/../examples/RAZ-RCP-1/http/minimal/server.js [Node.js]

<<< @/../examples/RAZ-RCP-1/http/minimal/main.go [Go]

<<< @/../examples/RAZ-RCP-1/http/minimal/index.php [PHP]

:::

Run it (with the same password as in the reader):

::: code-group

```bash [Python]
RCP_HASLO=secret python3 server.py
```

```bash [Node.js]
RCP_HASLO=secret node server.js
```

```bash [Go]
RCP_HASLO=secret go run main.go
```

```bash [PHP]
RCP_HASLO=secret php -S 0.0.0.0:8080 index.php
```

:::

The server listens on port 8080 and appends punches to `odbicia.tsv`
(`epoch<TAB>rfidID<TAB>machineId`).

Full examples in six languages (Python, Go, Node.js, Java, C#, PHP), with a
working time report and an employee list, are in
[`examples/RAZ-RCP-1/http`](https://github.com/ADARAZ-P-S-A/ADARAZ-INTEGRA/tree/master/examples/RAZ-RCP-1/http).

## 2. Reader setup

In the reader's panel: **Settings > Integrations > Synchronization**:

| Field | Value |
|---|---|
| Push punches to server | on |
| Format | `http3 - UID as hex` |
| Server URL | `http://<your computer's IP>:8080/odbicia` |
| Login / Password | `rcp` / the value of `RCP_HASLO` |
| Device ID (machineId) | any name, e.g. `Gate` |
| Interval [min] | 1 for testing, 15 by default |

With *Delete punches after upload* a punch accepted by the server (`200`) is
removed from the reader. Without it, it stays on the reader marked as sent.

## 3. Testing without a reader

The [reader simulator](https://github.com/ADARAZ-P-S-A/ADARAZ-INTEGRA/blob/master/examples/RAZ-RCP-1/http/symulator/symulator_czytnika.py)
(Python 3.9+, no dependencies) sends punches exactly like the reader and checks
your server's answers: wrong password, new punch, duplicate, unset clock, bad
data and special characters in `machineId`.

```bash
python3 symulator_czytnika.py http://127.0.0.1:8080/odbicia --haslo secret --test
```

```bash
python3 symulator_czytnika.py http://127.0.0.1:8080/odbicia --haslo secret --karta 04a1b2c3
```

`--test` also checks the `GET /raport` page of the full examples at the end. If
your server has no such page, ignore the last three checks.

## Protocol

The reader sends **each punch in a separate `POST`**, up to 32 punches per
cycle over one keep-alive connection:

```http
POST /odbicia HTTP/1.1
Authorization: Basic cmNwOnNlY3JldA==
Content-Type: application/x-www-form-urlencoded
User-Agent: RAZ-RCP/8

rfidID=04a1b2c3d4e5f6&timestamp=2026-10-28+06%3A58%3A12&machineId=Gate
```

| Field | Content |
|---|---|
| `rfidID` | Card number. `http3`: UID in lowercase hex (8 or 14 characters). `http2`: the same UID as a decimal number. Skryba: number computed like the old Skryba reader. |
| `timestamp` | `YYYY-MM-DD HH:MM:SS`, no zone: UTC in `http2`/`http3`, Polish time in Skryba. A year before 2026 means the reader's clock was not set. |
| `machineId` | Reader ID from its settings, may be empty. |

Full description in OpenAPI:
[`openapi.yaml`](https://github.com/ADARAZ-P-S-A/ADARAZ-INTEGRA/blob/master/examples/RAZ-RCP-1/http/openapi.yaml).

### What the reader does with your answer

| Answer | Reader |
|---|---|
| `200` | Punch delivered: marked as sent or deleted. Also with `"stored":false` in the body. |
| `400` | Punch rejected: **removed from the queue and never sent again**. |
| `401`, `403`, `404`, `5xx`, no answer | Cycle stopped, the punch stays in the reader. Retry after 30 s, then less often. |

### Rules for your server

1. **Answer `200` only when the punch is safely stored.** The reader may delete it at once.
2. **`400` only for a request you can never store.** When in doubt answer `5xx`:
   the reader retries later.
3. **The same punch may come twice**, e.g. when your answer was lost. Answer `200`
   (`{"stored":false,"reason":"duplicate"}`), otherwise the reader keeps sending
   it. The key is `(rfidID, timestamp, machineId)`.
4. **A punch with a year before 2026 was made with an unset clock.** Answering
   `{"stored":false,"reason":"clock"}` makes the reader log that.
5. **JSON without spaces.** The reader looks for exactly `"stored":false`.

## Server URL and HTTPS

- `http://` works only to a server in the local network (10.x, 172.16-31.x,
  192.168.x, 169.254.x, IPv6 fc/fd/fe80, or a name that points to such an
  address) and never over an open Wi-Fi.
- For a server on the internet use `https://` with a certificate from a public
  authority (e.g. Let's Encrypt), for example behind nginx or Caddy.
- Keep the URL to plain characters (letters, digits, `- . _ ~ /`). Some HTTP
  servers answer `400` to a URL with a stray `%`, and the reader drops such a punch.

## Before production

- `https://` in front of the server (reverse proxy with a public certificate),
- a database instead of the file, with a unique key on
  `(rfidID, timestamp, machineId)` and the same rule: store first, then `200`,
- separate credentials for each reader,
- direction (in/out) worked out on the server: the reader does not send it. The
  examples pair a person's punches in order (1st+2nd, 3rd+4th, ...), like the reader's panel.
