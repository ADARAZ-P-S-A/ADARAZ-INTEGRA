# RAZ-RCP-1

Proximity card reader and time clock. Every card tap is stored as a punch: the
card number and the time. It can connect to your system in three ways.

## Choose an integration

<div class="choice">
  <a class="choice-card" href="./http">
    <span class="choice-tag">simplest</span>
    <span class="choice-title">Punch push (HTTP)</span>
    <span class="choice-text">The reader sends every punch as a plain POST request to your server. On your side one HTTP endpoint in any language is enough.</span>
  </a>
  <a class="choice-card" href="./cloud/">
    <span class="choice-tag">like ADARAZ Cloud</span>
    <span class="choice-title">Own cloud server</span>
    <span class="choice-text">Your server instead of ADARAZ Cloud: pairing with a code, an employee list sent to the readers, punches, who is inside and unknown cards. A ready reference server in Go.</span>
  </a>
  <a class="choice-card" href="./link">
    <span class="choice-tag">live, both ways</span>
    <span class="choice-title">ADARAZ Link</span>
    <span class="choice-text">A permanent, encrypted connection between the reader and your server: live punches, reader state and remote relay control (e.g. opening a door).</span>
  </a>
</div>

## Comparison

| | Punch push (HTTP) | Own cloud server | ADARAZ Link |
|---|---|---|---|
| What you get | punches: card, time, reader ID | punches, unknown cards; the reader gets the employee list and who is inside from you | live punches, reader state, relay control |
| Employee list on the reader | set on the reader | from your server | set on the reader |
| When a punch arrives | every configured interval (1 minute or more) | every configured interval (1 minute or more) | right after the card tap |
| On your side | one `POST` endpoint | the ADARAZ Cloud API (`/device/v1`): 6 routes | a WebSocket server speaking Noise: a ready Python library |
| Security | `https://` (public certificate) or `http://` in the LAN, login and password | `https://` with a **short-lived certificate (up to 7 days)** or `http://` to an IP address in the LAN | mutual authentication with keys and Noise encryption, also over `ws://` |
| Pairing | you enter the URL, login and password | server address and a one-time code (8 characters) | server address and a one-time code (12 characters) |
| Reader settings | Settings > Integrations > Synchronization | Settings > Integrations > ADARAZ Cloud | Settings > Integrations > Home Assistant and other servers |
| Good for | systems that only collect punches | time and attendance or HR systems that manage employees and cards | access control, automation, Home Assistant |

Punch push (HTTP) and an own cloud server exclude each other: once paired with
a cloud server, it takes over the reader's synchronization settings.
ADARAZ Link can run alongside either.

## Common rules

- **The reader always connects out.** Your server never connects to the reader,
  so it needs no public address or open port.
- **No punch is lost.** The reader keeps punches until the server confirms it
  stored them. Without network it sends them later.
- **Time is UTC** (except the legacy Skryba formats of the HTTP push).
- **The reader does not know in from out.** It sends the punch only; your system
  works out the direction, e.g. by pairing a person's punches in order.
