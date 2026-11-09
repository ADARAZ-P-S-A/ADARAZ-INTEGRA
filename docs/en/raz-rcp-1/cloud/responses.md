# Responses and errors

The reader looks mostly at the status of the answer. An error with a body is JSON
`{"error":"<description>","code":"<stable code>"}`.

## What the reader does with an answer

| Answer | Reader |
|---|---|
| `200` | done (the punch is removed from the reader) |
| `304` | nothing changed (`/users`, `/presence`) |
| `400` | drops that one record, sends the rest |
| `401`, `403` | shows `Chmura odrzucila dane logowania - polacz ponownie`, punches wait on the reader |
| `404` | treats it as a wrong address: drops nothing, retries later |
| `429`, `5xx`, no answer | retries on the next sync |

## Rules for your server

- **No route answers `404` to a correct request** (except `/claim` with a wrong
  code). A `404` for an unknown card would block the reader's queue forever.
- **`400` only for a record you can never store.** The reader drops it.
- **`401` with the header `WWW-Authenticate: Basic realm="rcp"`** for a wrong
  login or password, also for a reader deleted in your system.
- **JSON without spaces**, no compressed answers.
- **An answer within 8 seconds.**

## Reconnecting a reader

When you take a reader's access away (new password or deletion), the reader stays
paired but every route gets `401`. To connect it again: **Disconnect** in the
reader's panel, then a new pairing code. Punches waiting on the reader then arrive
with the new credentials.

## `GET /hello`

A connection test:

```json
{"device":"Main entrance","organization":"My company","tz":"Europe/Warsaw","epoch":1793404800}
```

`epoch` is the server's time (unix seconds), useful to check the reader's clock.
