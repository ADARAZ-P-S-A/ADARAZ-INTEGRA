# RAZ-RCP-1 - punch push (HTTP) examples

Receive punches from the RAZ-RCP-1 reader on your own server, the "Punch push
(HTTP)" integration. Full description on the portal:
https://integra.adaraz.com/en/raz-rcp-1/http

| Path | Content |
|---|---|
| `minimal/` | the smallest complete servers shown on the portal (Python, Node.js, Go, PHP) |
| `python/`, `go/`, `nodejs/`, `java/`, `csharp/`, `php/` | the same time clock server with a working time report, in six languages |
| `symulator/` | a reader simulator to test any server, and a script that tests all six examples |
| `openapi.yaml` | the contract in OpenAPI 3.0 |


The RAZ-RCP-1 reader can send every card punch to your server. This directory shows
how to receive them: the same small time clock server in six languages, a reader
simulator to test your own server, and an OpenAPI 3.0 description of the contract.

Each example:

- accepts punches exactly as the reader sends them (login and password checked),
- stores them in a text file and writes them to disk before answering,
- recognises duplicates and punches made with an unset reader clock,
- shows a working time report: who is at work now, monthly totals and shifts.

They have no dependencies beyond the language itself, so you can read one file
and copy what you need into your own system.

## Examples

| Language | Folder | Run | Requires |
|---|---|---|---|
| Python | [`python/`](python/server.py) | `RCP_HASLO=secret python3 server.py` | Python 3.9+ |
| Go | [`go/`](go/main.go) | `RCP_HASLO=secret go run .` | Go 1.21+ |
| Node.js | [`nodejs/`](nodejs/server.js) | `RCP_HASLO=secret node server.js` | Node.js 18+ |
| Java | [`java/`](java/RcpServer.java) | `RCP_HASLO=secret java RcpServer.java` | JDK 17+ |
| C# | [`csharp/`](csharp/Program.cs) | `RCP_HASLO=secret dotnet run` | .NET 10 SDK |
| PHP | [`php/`](php/index.php) | `RCP_HASLO=secret php -S 0.0.0.0:8080 index.php` | PHP 8.1+ with mbstring |

Run the command inside the folder. On Windows (PowerShell) set the variable first:
`$env:RCP_HASLO="secret"`. Python on Windows also needs `pip install tzdata`.
Java reads environment variables in the system locale: with a password that has
non-ASCII characters, run it with `LANG=C.UTF-8`.

PHP under Apache or nginx: the folder with `index.php` is served by the web server,
so set `RCP_DANE` and `RCP_PRACOWNICY` to paths outside the web root (the example
refuses to start without `RCP_DANE` there). The URL in the reader then points at
`index.php`.

The server listens on port 8080. Open `http://localhost:8080/raport` in a browser
(login `rcp`, the password you set) to see the report.

## Reader setup

In the reader's panel: **Settings > Integrations > Synchronization**:

| Field | Value |
|---|---|
| Push punches to server | on |
| Format | `http3 - UID as hex` |
| Server URL | `http://<your computer's IP>:8080/odbicia` |
| Login / Password | `rcp` / the value of `RCP_HASLO` |
| Device ID (machineId) | any name, e.g. `Gate` - shown in the server log |
| Interval [min] | 1 for testing, 15 by default |

`http://` works only to a server in the local network (10.x, 172.16-31.x, 192.168.x,
169.254.x, IPv6 fc/fd/fe80, or a name that points to such an address) and never over
an open Wi-Fi. For a server on the internet use `https://` with a certificate from a
public authority (e.g. Let's Encrypt), for example behind nginx or Caddy.

Keep the server URL to plain characters (letters, digits, `- . _ ~ /`). Some HTTP
servers, e.g. the one built into Java, answer `400` to a URL with a stray `%`, and
the reader drops a punch answered with `400`.

## Configuration

Every example reads the same environment variables:

| Variable | Default | Meaning |
|---|---|---|
| `RCP_HASLO` | (required) | Password, the same as in the reader |
| `RCP_LOGIN` | `rcp` | Login, the same as in the reader |
| `RCP_PORT` | `8080` | HTTP port (PHP: given in the `php -S` command) |
| `RCP_DANE` | `odbicia.tsv` in the current folder | Punch file |
| `RCP_PRACOWNICY` | `pracownicy.csv` | Optional employee list for the report |
| `RCP_STREFA_CZYTNIKA` | `UTC` | Time zone of `timestamp`: `UTC` for http2/http3, `Europe/Warsaw` for the Skryba formats |
| `RCP_STREFA` | `Europe/Warsaw` | Time zone of the report |

Names in the report come from `pracownicy.csv` (see
[`pracownicy.przyklad.csv`](pracownicy.przyklad.csv)): one `rfidID;Name` per line,
several cards with the same name count as one person. Cards not on the list show
as `karta <rfidID>`. Save the file as UTF-8 (in Excel: *CSV UTF-8*).

In the Skryba formats the reader sends Polish local time, so the hour repeated when
clocks go back in October is ambiguous. The examples take the later one (winter time).

The punch file has one line per punch: `epoch<TAB>rfidID<TAB>machineId`. The format
is the same in all six examples: point `RCP_DANE` at the same file and you can switch
languages without losing data.

## The protocol in short

Full description: [`openapi.yaml`](openapi.yaml).

The reader sends **each punch in a separate `POST`** to the configured URL, up to
32 punches per cycle over one keep-alive connection:

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
| `timestamp` | `YYYY-MM-DD HH:MM:SS`, no zone: UTC in http2/http3, Polish time in Skryba. Before 2026 = the reader's clock was not set. |
| `machineId` | Reader ID from its settings, may be empty. |

What the reader does with your answer:

| Answer | Reader |
|---|---|
| `200` | Punch delivered - marked as sent (or deleted, when *Delete punches after upload* is on). Also with `"stored":false` in the body. |
| `400` | Punch rejected - **removed from the queue and never sent again**. |
| `401`, `403`, `404`, `5xx`, no answer | Cycle stopped, punch stays in the reader, retry after 30 s, then less often. |

Rules for your server:

1. Answer `200` only when the punch is safely stored - the reader may delete it at once.
2. Answer `400` only for a request you can never store. When in doubt, answer `5xx`.
3. The same punch may come twice (e.g. when your answer was lost). Answer `200`
   (`{"stored":false,"reason":"duplicate"}`), otherwise the reader keeps sending it.
4. A punch with a year before 2026 was made with an unset clock. Answering
   `{"stored":false,"reason":"clock"}` makes the reader log that.

Who goes in and who goes out is not sent - the reader does not know it either.
The examples pair a person's punches in order (1st+2nd, 3rd+4th, ...) without
cutting at midnight, the same rule as the reader's own panel.

## Testing your server

[`symulator/symulator_czytnika.py`](symulator/symulator_czytnika.py) (Python 3.9+,
no dependencies) sends punches exactly like the reader and checks the answers:

```bash
python3 symulator/symulator_czytnika.py http://127.0.0.1:8080/odbicia --haslo secret --test
```

```bash
python3 symulator/symulator_czytnika.py http://127.0.0.1:8080/odbicia --haslo secret --karta 04a1b2c3
```

`--format http2|http3|skryba8|skryba10` selects the format. The `--test` mode also
checks `GET /raport` - for your own server without that page, ignore the last three checks.
[`symulator/test_wszystkie.sh`](symulator/test_wszystkie.sh) runs the test against
all six examples.

## Before production

The examples are meant to be read and adapted. For real use, consider:

- `https://` in front of the server (reverse proxy with a public certificate),
- a database instead of the text file (keep the rule: store first, then answer `200`),
- a unique key on `(rfidID, timestamp, machineId)` to catch duplicates,
- separate credentials for each reader, and for the report page.

