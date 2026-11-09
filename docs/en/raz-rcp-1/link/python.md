# Your own server in Python

The [`adaraz`](https://pypi.org/project/adaraz/) library (Python 3.12+) is a
complete implementation of the protocol: pairing, sessions, encryption, punch
acknowledgements and relay control. You get events.

```bash
pip install "adaraz[aiohttp]"
```

The smallest working server:

<<< @/../examples/RAZ-RCP-1/link/server.py

Run it and it prints a pairing code:

```
pairing code: 7QPM-36QP-WNXQ  server URL: http://<this computer>:8765
```

## Pairing the reader

In the reader's panel: **Settings > Integrations > Home Assistant and other servers**:

| Field | Value |
|---|---|
| Server address | `http://<your computer's IP>:8765` |
| Code | the code printed by the server, e.g. `7QPM-36QP-WNXQ` (valid 10 minutes, one reader) |

Click **Connect**. Letter case, the dashes and the letters O / I / L (read as
0 / 1 / 1) do not matter. Once connected, the panel shows the **Server ID**:
compare it with `server.server_id` on the server.

## Relay

The server can drive the relay only after the reader's administrator enables
**Let the server control the relay** in the same panel section.

```python
result = await server.send_relay(device_id, "open")          # pulse like an accepted card
result = await server.send_relay(device_id, "pulse", ms=1500)  # 1.5 s pulse
result = await server.send_relay(device_id, "on")             # on; "off" turns it off
if not result.ok:
    print(result.err)  # not_allowed (no consent), invalid, busy
```

## Before production

- **Keep the server key and the paired readers.** A reader pins the server's
  public key at pairing: a new key after a restart means pairing every reader
  again. Implement `LinkStorage` (devices, cursor, storing punches) instead of
  `InMemoryStorage`. A file-based version:
  [`examples/server.py`](https://github.com/ADARAZ-P-S-A/ADARAZ-HA/blob/master/examples/server.py).
- **Late punches** (`punch.late`, arrived more than 5 minutes after the tap):
  keep them in history but do not use them for live actions such as opening a door.
- **Every punch arrives once:** the server acknowledges a batch only after
  `store_punches` stored the punches and the cursor. Duplicates (the same card
  in the same second) are filtered by `PunchDeduplicator`.

The full library API: [library README](https://github.com/ADARAZ-P-S-A/ADARAZ-HA/blob/master/lib/README.md).

## Testing without a reader {#test}

The library has a simulated reader (`adaraz.device.LinkDevice`) that behaves
like the real one: pairs with a code, connects, sends punches and executes
relay commands.

```python
import asyncio, aiohttp
from adaraz.aiohttp_ws import connect
from adaraz.device import LinkDevice

async def main(code: str) -> None:
    async with aiohttp.ClientSession() as http:
        reader = LinkDevice(device_id="C00F0202")
        await reader.pair(await connect(http, "http://127.0.0.1:8765"), code)
        session = asyncio.create_task(reader.run_session(await connect(http, "http://127.0.0.1:8765")))
        reader.punch("04A1B2C3")
        await reader.wait_delivered()
        await reader.stop()
        await session

asyncio.run(main("7QPM-36QP-WNXQ"))
```
