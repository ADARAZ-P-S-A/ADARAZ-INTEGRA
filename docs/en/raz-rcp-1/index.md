# RAZ-RCP-1

Proximity card reader and time clock. Every card tap is stored as a punch: the
card number and the time. The reader can pass punches to your system in two ways.

## Choose an integration

<div class="choice">
  <a class="choice-card" href="./http">
    <span class="choice-tag">simplest</span>
    <span class="choice-title">Punch push (HTTP)</span>
    <span class="choice-text">The reader sends every punch as a plain POST request to your server. On your side one HTTP endpoint in any language is enough.</span>
  </a>
  <a class="choice-card" href="./link">
    <span class="choice-tag">both ways</span>
    <span class="choice-title">ADARAZ Link</span>
    <span class="choice-text">A permanent, encrypted connection between the reader and your server: live punches, reader state and remote relay control (e.g. opening a door).</span>
  </a>
</div>

## Comparison

| | Punch push (HTTP) | ADARAZ Link |
|---|---|---|
| Direction | the reader sends punches, the server only answers | permanent connection, messages both ways |
| What you get | punches: card, time, reader ID | live punches, reader state, relay control |
| When a punch arrives | every configured interval (1 minute or more) | moments after the card tap |
| On your side | any HTTP server, one `POST` endpoint | a WebSocket server speaking Noise: a ready Python library or your own implementation |
| Security | `https://` with a public CA certificate, or `http://` inside the LAN, login and password | mutual authentication with keys and Noise encryption, also over `ws://` in the LAN |
| Pairing | you enter the URL, login and password | a one-time code from the server, valid 10 minutes |
| Reader settings | Settings > Integrations > Synchronization | Settings > Integrations > Home Assistant and other servers |
| Good for | time and attendance or HR systems that collect punches | access control, automation, Home Assistant, real-time reactions |

Both can run at the same time: the reader can push punches over HTTP to an HR
system and be connected over ADARAZ Link to an automation server.

## Common rules

- **The reader always connects out.** Your server never connects to the reader,
  so it needs no public address or open port.
- **No punch is lost.** The reader keeps punches until the server confirms it
  stored them. Without network it sends them later.
- **Time is UTC** (except the legacy Skryba formats of the HTTP push).
- **The reader does not know in from out.** It sends the punch only; your system
  works out the direction, e.g. by pairing a person's punches in order.
