# Employee list: `GET /users`

The reader takes the list of cards it knows (the users.bin file) from your
server. After pairing this is the only source of the list: it is read-only in
the reader's panel.

## Request

```http
GET /device/v1/users
Authorization: Basic ...
If-None-Match: "e3b0c44298fc1c149afbf4c8996fb924"
```

`If-None-Match` is the hash of the file the reader has: the first 16 bytes of
SHA-256 of its content, lowercase hex. An empty file:
`e3b0c44298fc1c149afbf4c8996fb924`. The reader may add a second value after a
comma: the hash of a list it turned down earlier (e.g. too long), so that it does
not download it every minute.

## Answer

- the hash of your list equals any value of `If-None-Match`: `304` with no body,
- otherwise `200`, `Content-Type: application/octet-stream`, the body is the list.

Both carry `ETag: "<hash of the list>"` and `X-RCP-Users: <number of records>`.
An empty list is a `200` with an empty body.

## Format

48-byte records, one per card:

| Bytes | |
|---|---|
| `[0]` | `0x00` |
| `[1]` | UID length, 4-7 |
| `[2..8]` | UID, zero-padded to 7 bytes |
| `[9..45]` | the person's number (`emp_no`) as ASCII digits (1-20), zero bytes after it |
| `[46..47]` | CRC-16/CCITT-FALSE of bytes `[0..45]`, little-endian |

CRC-16/CCITT-FALSE: polynomial `0x1021`, initial value `0xFFFF`, no reflection,
no final XOR. For `"123456789"` it is `0x29B1`.

- Order: by `emp_no` as a number, then by the UID in hex. The same list always
  gives the same bytes, and so the same hash.
- `emp_no` is the person's permanent number. Two cards of one person are two
  records with the same `emp_no`: the reader pairs entries and exits by it.
- No names: the reader does not need personal data.
- At most 2048 records (98,304 bytes). A longer list is turned down and the reader keeps its old one.

## How the reader checks the list

One bad record rejects the whole list (the reader keeps the previous one): a size
not divisible by 48, `X-RCP-Users` not matching the number of records, a wrong CRC,
or a UID length outside 4-7.

The reader asks at most every 50 seconds, and right after pairing and after a restart.
