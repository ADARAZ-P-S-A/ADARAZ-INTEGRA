# Pairing: `POST /claim`

The reader exchanges a one-time pairing code for a login, a password and
addresses. It is the only route without HTTP Basic: the code is the credential.

## Pairing code

- 8 characters of `23456789ABCDEFGHJKLMNPQRSTUVWXYZ` (no 0/O, no 1/I), e.g. `ABCD-EFGH`,
- valid 15 minutes, single use,
- accept it in any letter case, with or without the dash, with spaces.

## Request

```http
POST /device/v1/claim
Content-Type: application/json

{"code":"ABCD-EFGH","serial":"ef4924d4-4b54-433b-983a-14b62cc8eaeb","firmware":"121"}
```

`serial` is the reader's permanent identifier, `firmware` its software version.

## Answer

```json
{"device":{"id":"...","name":"RAZ-RCP-1 ef4924d4-..."},
 "organization":"My company",
 "credentials":{"url":"https://rcp.example.com/device/v1/punch","login":"rcp-3fa2c1d0e9b8",
                "password":"<43 characters>","format":3,"interval_min":1},
 "api":"https://rcp.example.com/device/v1"}
```

| Field | |
|---|---|
| `organization`, `device.name` | shown in the reader's panel (up to 63 bytes) |
| `credentials.url` | the punch address: `api` + `/punch` |
| `credentials.login`, `password` | HTTP Basic for the other routes (each shorter than 64 characters) |
| `format` | always `3`: the UID as hex |
| `interval_min` | how often the reader syncs, in minutes (1-1440) |
| `api` | the base of every other route |

- `url` and `api` must meet the [address requirements](./requirements); the whole
  answer must be shorter than 2048 bytes.
- Do not store the password in plain text: keep a hash (e.g. SHA-256 of 32 random bytes).

## Errors

- A wrong, used or expired code: `404 {"code":"bad_pairing_code"}`.
  This is the only route where `404` is a correct answer.
- The reader retries pairing up to 3 times when it gets no answer or a `5xx`.

## Retried pairing

A code works once. The exception: the answer never reached the reader, so the
reader asks again with the same code and the same `serial`. If it does so within
15 minutes and before it used its credentials, answer `200` with the same reader
and login but a **new** password. The old password then stops working. Another
`serial`, or a reader that has already logged in: `404`.
