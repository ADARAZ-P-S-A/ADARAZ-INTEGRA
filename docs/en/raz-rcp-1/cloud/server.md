# Reference server

A complete server in Go (standard library only, data in one JSON file):
[`examples/RAZ-RCP-1/cloud`](https://github.com/ADARAZ-P-S-A/ADARAZ-INTEGRA/tree/master/examples/RAZ-RCP-1/cloud).
Meant to be read and moved into your own system.

## Running

```bash
cd examples/RAZ-RCP-1/cloud
PUBLIC_URL=https://rcp.example.com ADMIN_TOKEN=$(openssl rand -hex 24) go run .
```

`PUBLIC_URL` is exactly the address you type into the reader (see
[Requirements](./requirements)). Other variables: `LISTEN` (`:8080`),
`DATA_FILE` (`data.json`), `ORG_NAME`, `TZ_NAME`.

The server's routes:

<<< @/../examples/RAZ-RCP-1/cloud/main.go#routes{go}

| File | Content |
|---|---|
| `device.go` | the reader's API (`/device/v1`) |
| `admin.go` | your system's API (`/admin`) |
| `protocol.go` | pairing codes, UIDs, time, users.bin, CRC, ETags |
| `store.go` | the data and its file |
| `e2e_test.go` | test: your system and a reader over real HTTP |
| `logging.go` | one log line per request |

Every request is one log line: what came in, the answer, and what happened:

```
POST /device/v1/punch 200 1ms 192.168.1.50:51234 device="Entrance" uid=04a1b2c3 ts=2026-11-09T06:58:12Z stored employee=E001
POST /device/v1/punch 200 0s 192.168.1.50:51234 device="Entrance" uid=04a1b2c3 ts=2000-01-01T00:14:55Z not stored: the reader's clock was not set at the tap
GET /device/v1/users 304 0s 192.168.1.50:51234 device="Entrance" employee list unchanged (2 cards)
```

## Pairing a reader

A pairing code (8 characters, valid 15 minutes, single use):

```bash
curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" https://rcp.example.com/admin/pairing
```

```json
{"code":"ABCD-EFGH","expires_at":"2026-11-09T10:15:00Z"}
```

In the reader's panel: **Settings > Integrations > ADARAZ Cloud**:

1. **Own cloud server > Server address**: `https://rcp.example.com`, save.
2. **Pairing code**: `ABCD-EFGH`, **Connect**.

The reader shows "Connected to organisation ... as ...". From then on it sends
punches by itself, and its local employee list is read-only: it comes from your server.

## Employees and cards

You send the whole list at once, and the readers take it on their next sync:

```bash
curl -X PUT -H "Authorization: Bearer $ADMIN_TOKEN" https://rcp.example.com/admin/employees \
  -d '[{"id":"E001","name":"Jan Kowalski","cards":["04a1b2c3"]},
       {"id":"E002","name":"Anna Nowak","cards":["04d4e5f6","0477889900aabb"]}]'
```

Names never reach the reader, only the person's number and their cards.

## Punches

```bash
curl -H "Authorization: Bearer $ADMIN_TOKEN" "https://rcp.example.com/admin/punches?after=0"
```

```json
[{"id":1,"device_id":"...","uid":"04a1b2c3","employee_id":"E001","ts":"2026-11-09T06:58:12Z","received":"2026-11-09T06:58:40Z"}]
```

Fetch the next ones with `after=<last id>`. Unknown cards: `GET /admin/seen`.
Readers: `GET /admin/devices`; `DELETE /admin/devices/{id}` cuts a reader off
(it gets `401` on its next request).

## Test

```bash
cd examples/RAZ-RCP-1/cloud && go test ./...
```

The test runs the real HTTP server and plays both sides, your system and a reader
(pairing, the employee list with CRC and `304`, punches, duplicate, unset clock,
presence, unknown card, wrong password, deleted reader, retried pairing).

Simplification of the reference server: a punch belongs to whoever holds the card
when the punch reaches the server.
