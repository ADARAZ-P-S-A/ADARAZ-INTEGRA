# Punch push (HTTP)

The simplest integration: every configured interval the reader sends new
punches to your server's URL, each in its own `POST` request. Your server
stores the punch and answers `200`. Nothing else is needed.

```
reader ── POST rfidID, timestamp, machineId ──▶ your server
       ◀────────── 200 {"stored":true} ─────────
```

## Contents

| Page | What is on it |
|---|---|
| [The server](./server) | a complete server in Python, Node.js, Go and PHP; full examples in 6 languages |
| [Reader setup](./reader) | what to enter in the reader's panel, address and HTTPS rules |
| [Protocol](./protocol) | the request, fields, what the reader does with your answer, rules for the server |
| [Testing and production](./testing) | the reader simulator, a checklist before going live |

## In short

1. Run a [server](./server) that stores the punch on `POST` and answers `200`.
2. Enter its address, login and password in the reader ([reader setup](./reader)).
3. Check it with the [reader simulator](./testing).
