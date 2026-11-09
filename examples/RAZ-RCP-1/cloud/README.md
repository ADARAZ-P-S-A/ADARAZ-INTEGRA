# Cloud server - reference implementation

A small server that speaks the ADARAZ Cloud device API (`/device/v1`). Point a
RAZ-RCP-1 reader at it as its own cloud server and the reader works with it the
same way it works with ADARAZ Cloud: it pairs with a code, takes the employee
list from it, sends punches, asks who is inside and reports unknown cards.

Go standard library only, data in one JSON file. Meant to be read and copied
into your own system. The full description is on the portal:
https://integra.adaraz.com/en/raz-rcp-1/cloud

| File | Content |
|---|---|
| `main.go` | routes and server start |
| `device.go` | what the reader calls (`/device/v1`) |
| `admin.go` | what your system calls (`/admin`) |
| `protocol.go` | pairing codes, UIDs, time, users.bin, CRC, ETags |
| `store.go` | the data and its JSON file |
| `e2e_test.go` | end-to-end test: your system and a reader over real HTTP |

## Run

```bash
PUBLIC_URL=http://192.168.1.10:8080 ADMIN_TOKEN=$(openssl rand -hex 24) go run .
```

`PUBLIC_URL` is exactly the address typed into the reader. Other variables:
`LISTEN` (`:8080`), `DATA_FILE` (`data.json`), `ORG_NAME`, `TZ_NAME`.

```bash
go test ./...
```

## Requirements of the reader

- `https://` with a **short-lived certificate (valid for at most 7 days)** from
  a public CA, e.g. Let's Encrypt with the `shortlived` profile; or `http://` to
  a private IP address in the local network (10.x, 172.16-31.x, 192.168.x), not
  a host name.
- `PUBLIC_URL` + `/device/v1/punch` shorter than 128 characters.

Details: https://integra.adaraz.com/en/raz-rcp-1/cloud#requirements

## API

Device (HTTP Basic from the `/claim` answer):

| | |
|---|---|
| `POST /device/v1/claim` | `{code, serial, firmware}` -> login, password, addresses |
| `POST /device/v1/punch` | form `rfidID`, `timestamp` (UTC) |
| `GET /device/v1/users` | users.bin (48 bytes per card), 304 when the reader has it |
| `GET /device/v1/presence` | who is inside, one bit per users.bin record |
| `POST /device/v1/seen` | an unknown card (`rfidID`, `timestamp`, `denied`) |
| `GET /device/v1/hello` | connection test |

Your system (`Authorization: Bearer ADMIN_TOKEN`):

| | |
|---|---|
| `POST /admin/pairing` | a new pairing code |
| `PUT /admin/employees` | `[{id, name, cards: ["04a1b2c3"]}]` - replaces the whole list |
| `GET /admin/employees` | the list |
| `GET /admin/punches?after=<id>` | punches after that id |
| `GET /admin/devices` | readers |
| `DELETE /admin/devices/{id}` | disconnects a reader (401 on its next request) |
| `GET /admin/seen` | unknown cards to assign |

Simplification: a punch belongs to whoever holds the card when it arrives.
