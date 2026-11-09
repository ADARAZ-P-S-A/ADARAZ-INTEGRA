# Punches and unknown cards

## `POST /punch`

The reader sends each punch in its own request, up to 32 per cycle, over one
keep-alive connection:

```http
POST /device/v1/punch
Authorization: Basic ...
Content-Type: application/x-www-form-urlencoded

rfidID=04a1b2c3&timestamp=2026-11-09+06%3A58%3A12&machineId=RAZ-RCP-1
```

| Field | |
|---|---|
| `rfidID` | the card's UID in hex, 4-7 bytes (letter case, `:`, `-` and spaces do not matter) |
| `timestamp` | `YYYY-MM-DD HH:MM:SS` in UTC |
| `machineId` | ignore it: identify the reader by its login |

### Answers

| Answer | When |
|---|---|
| `200 {"stored":true,"duplicate":false}` | stored |
| `200 {"stored":false,"duplicate":true}` | the same punch again (key: reader, card, time) |
| `200 {"stored":false,"reason":"clock"}` | time before 2020 or more than a day ahead: the reader's clock was not set |
| `400 {"code":"bad_uid"}` | `rfidID` is not a hex UID |
| `400 {"code":"bad_time"}` | `timestamp` cannot be read |

- **Answer `200` only after storing.** After a `200` the reader deletes the punch.
- **An unknown card is a `200` too.** The punch is real; assign the card later.
  A `404` or `5xx` would block the reader's queue.
- **A duplicate is a `200`.** When your answer gets lost, the reader sends the punch again.
- **JSON without spaces.** The reader looks for exactly `"stored":false`.
- A punch belongs to whoever held the card at the moment of the punch (`timestamp`).

## `POST /seen`

The reader reports at once a card that is not on its list, including one it
refused, for which it sent no punch. This lets you tap a new card and assign it
in your system.

```http
POST /device/v1/seen
Authorization: Basic ...
Content-Type: application/x-www-form-urlencoded

rfidID=04c1c2c3&timestamp=2026-11-09+07%3A01%3A00&denied=1
```

`denied=1`: the reader refused it; `0`: it let it through and recorded a punch.

| Answer | When |
|---|---|
| `200 {"stored":true}` | stored |
| `200 {"stored":false,"reason":"clock"}` | the reader's clock was not set |
| `400` | `rfidID` or `timestamp` cannot be read |

Once the card is assigned in your system, it goes into the next
[employee list](./users).
