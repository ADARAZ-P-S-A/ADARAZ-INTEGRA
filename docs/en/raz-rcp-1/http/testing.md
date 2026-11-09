# Testing and production

## Testing without a reader

The [reader simulator](https://github.com/ADARAZ-P-S-A/ADARAZ-INTEGRA/blob/master/examples/RAZ-RCP-1/http/symulator/symulator_czytnika.py)
(Python 3.9+, no dependencies) sends punches exactly like the reader and checks
your server's answers: wrong password, new punch, duplicate, unset clock, bad
data and special characters in `machineId`.

```bash
python3 symulator_czytnika.py http://127.0.0.1:8080/odbicia --haslo secret --test
```

```bash
python3 symulator_czytnika.py http://127.0.0.1:8080/odbicia --haslo secret --karta 04a1b2c3
```

`--test` also checks the `GET /raport` page of the full examples at the end. If
your server has no such page, ignore the last three checks.

## Before production

- `https://` in front of the server (reverse proxy with a public certificate),
- a database instead of the file, with a unique key on
  `(rfidID, timestamp, machineId)` and the same rule: store first, then `200`,
- separate credentials for each reader,
- direction (in/out) worked out on the server: the reader does not send it. The
  examples pair a person's punches in order (1st+2nd, 3rd+4th, ...), like the reader's panel.
