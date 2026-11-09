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
| [Home Assistant](./home-assistant) | a ready integration, no code |
| [Your own server in Python](./python) | the `adaraz` library does the whole protocol, you only handle punches |
| [Another language](./other-languages) | your own implementation of the specification |
