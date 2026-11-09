# ADARAZ Link

A permanent, encrypted connection between the reader and your server. The
reader connects to the server over a WebSocket by itself and keeps it open:

- punches arrive right after the card tap; those made while offline are
  delivered later and marked as late,
- the server sees the reader's state (connected, card reader working, relay state),
- the server can drive the relay, e.g. open a door, if the reader's
  administrator allows it.

```
reader ══ WebSocket + Noise (connects out) ══ your server
  punches, state ────────────────────────────▶
                 ◀──────────────────────────── acks, relay
```

Every connection is a [Noise](https://noiseprotocol.org/) session with mutual
authentication by keys exchanged at pairing, so plain `ws://` inside a LAN is
safe. `wss://` works too, for a server on the internet.

## Three ways

| | For whom |
|---|---|
| [Home Assistant](#home-assistant) | a ready integration, no code |
| [Your own server in Python](#python) | the `adaraz` library does the whole protocol, you only handle punches |
| [Another language](#another-language) | your own implementation of the specification |

## Home Assistant {#home-assistant}

The ADARAZ integration for Home Assistant: punches as events, a connection
sensor, a relay button and switch. Installed through HACS, described in the
[ADARAZ-HA](https://github.com/ADARAZ-P-S-A/ADARAZ-HA) repository.

## Your own server in Python {#python}

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

### Pairing the reader

In the reader's panel: **Settings > Integrations > Home Assistant and other servers**:

| Field | Value |
|---|---|
| Server address | `http://<your computer's IP>:8765` |
| Code | the code printed by the server, e.g. `7QPM-36QP-WNXQ` (valid 10 minutes, one reader) |

Click **Connect**. Letter case, the dashes and the letters O / I / L (read as
0 / 1 / 1) do not matter. Once connected, the panel shows the **Server ID**:
compare it with `server.server_id` on the server.

### Relay

The server can drive the relay only after the reader's administrator enables
**Let the server control the relay** in the same panel section.

```python
result = await server.send_relay(device_id, "open")          # pulse like an accepted card
result = await server.send_relay(device_id, "pulse", ms=1500)  # 1.5 s pulse
result = await server.send_relay(device_id, "on")             # on; "off" turns it off
if not result.ok:
    print(result.err)  # not_allowed (no consent), invalid, busy
```

### Before production

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

### Testing without a reader

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

## Another language {#another-language}

The protocol is fully described in the [ADARAZ Link specification](./link-protocol).
In short:

1. **Transport:** WebSocket at `/api/adaraz/ws`, subprotocol `adaraz.v1`, binary
   messages only, one WebSocket message = one Noise message (up to 4096 bytes).
2. **Pairing:** `Noise_XXpsk3_25519_AESGCM_SHA256`, the PSK derived from the
   12-character code (HKDF-SHA256). The server stores the reader's public key,
   the reader stores the server's.
3. **Session:** `Noise_IK_25519_AESGCM_SHA256`. The reader sends `hello`, the
   server answers `welcome` with the cursor the reader sends punches from.
4. **Punches:** `punches` messages (up to 100 items `[index, uid_hex, unix_ts]`),
   the server answers `ack` only after storing them durably.
5. **Relay:** `relay` -> `relay_result`, only with the reader administrator's consent.

To test your implementation, use the simulated reader from the `adaraz`
library (above): an independent, tested implementation of the reader side.
