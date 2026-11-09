# Own cloud server

The reader connects to your server exactly as it connects to ADARAZ Cloud. Your
server implements the ADARAZ Cloud device API (`/device/v1`), and the reader:

- pairs with it using a one-time code and sets up sending punches by itself,
- takes the **employee list and their cards** from it (who to let in, how to count entries),
- sends punches,
- asks **who is inside**, so a person can enter through one reader and leave through another,
- reports **unknown cards**, so they can be assigned in your system.

```
reader ── punches, unknown cards ───────────▶ your server
       ◀── employee list, who is inside ─────
```

In the reader's panel this is the **Own cloud server** field in the ADARAZ Cloud
section. The reader always connects out, every configured interval (1 minute or
more); the server never connects to the reader.

::: warning Requirements first
The reader accepts `https://` only with a **short-lived certificate, valid for
at most 7 days**. A regular 90-day certificate will not work.
Details: [Requirements](./requirements).
:::

## Contents

| Page | What is on it |
|---|---|
| [Requirements](./requirements) | short-lived certificate, `http://` in the LAN, address rules |
| [Reference server](./server) | a ready server in Go: running, pairing, employees, punches, test |
| [Pairing](./pairing) | `POST /claim`: code -> login, password, addresses |
| [Employee list](./users) | `GET /users`: users.bin format, CRC, `304` |
| [Punches and unknown cards](./punches) | `POST /punch`, `POST /seen` |
| [Presence](./presence) | `GET /presence`: who is inside |
| [Responses and errors](./responses) | what the reader does with each status, `GET /hello` |

## One reader cycle

```
1. POST /seen       unknown cards from the queue (right after the tap)
2. GET  /users      at most every 50 s: the employee list or 304
3. POST /punch      up to 32 punches, one per request, over one connection
4. GET  /presence   at most every 20 s: who is inside or 304
```

Every route except `/claim` needs HTTP Basic with the login and password from the
`/claim` answer. After pairing the reader's local employee list is read-only, and
punches accepted by the server are removed from the reader.
