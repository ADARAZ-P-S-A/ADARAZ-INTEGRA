# Protocol

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

## What the reader does with your answer

| Answer | Reader |
|---|---|
| `200` | Punch delivered: marked as sent or deleted. Also with `"stored":false` in the body. |
| `400` | Punch rejected: **removed from the queue and never sent again**. |
| `401`, `403`, `404`, `5xx`, no answer | Cycle stopped, the punch stays in the reader. Retry after 30 s, then less often. |

## Rules for your server

1. **Answer `200` only when the punch is safely stored.** The reader may delete it at once.
2. **`400` only for a request you can never store.** When in doubt answer `5xx`:
   the reader retries later.
3. **The same punch may come twice**, e.g. when your answer was lost. Answer `200`
   (`{"stored":false,"reason":"duplicate"}`), otherwise the reader keeps sending
   it. The key is `(rfidID, timestamp, machineId)`.
4. **A punch with a year before 2026 was made with an unset clock.** Answering
   `{"stored":false,"reason":"clock"}` makes the reader log that.
5. **JSON without spaces.** The reader looks for exactly `"stored":false`.
