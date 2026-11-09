# Presence: `GET /presence`

A person may enter through one reader and leave through another, and each reader
knows only its own punches. So after sending its punches the reader asks who is
inside, and uses that for the direction of the person's next punch.

## Request

```http
GET /device/v1/presence
Authorization: Basic ...
If-None-Match: "<ETag of the answer it applied last>"
X-RCP-Users: "<hash of the reader's users.bin>"
```

`If-None-Match` is empty after the reader starts. `X-RCP-Users` is for logs only.

## Answer

One bit per record of the [employee list](./users), in the same order:

- record `i` is byte `i/8`, bit `i%8`, bit 0 the least significant,
- `1` = the card's holder is inside,
- `ceil(N/8)` bytes long; an empty list = an empty body.

| Header | |
|---|---|
| `X-RCP-Users-ETag` | the hash of the list the bits are about |
| `ETag` | the first 16 bytes of SHA-256 of (the list hash as 32 hex characters, then the bits), hex |

- a matching `If-None-Match`: `304` with the same headers,
- otherwise `200`, `Content-Type: application/octet-stream`, the body is the bits.

## Rules

- **Inside** = an odd number of the person's punches (from every reader, deleted ones aside).
  Both cards of one person have the same bit.
- **A punch accepted a moment ago must already be in the answer.** The reader asks
  right after sending punches and trusts the answer.
- The reader applies the bits only when `X-RCP-Users-ETag` equals the hash of its
  list. Otherwise it waits for the next sync.
- The reader asks at most every 20 seconds, and not at all when its list has more than 256 cards.
