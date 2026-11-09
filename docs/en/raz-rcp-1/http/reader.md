# Reader setup

In the reader's panel: **Settings > Integrations > Synchronization**:

| Field | Value |
|---|---|
| Push punches to server | on |
| Format | `http3 - UID as hex` |
| Server URL | `http://<your computer's IP>:8080/odbicia` |
| Login / Password | `rcp` / the value of `RCP_HASLO` |
| Device ID (machineId) | any name, e.g. `Gate` |
| Interval [min] | 1 for testing, 15 by default |

With *Delete punches after upload* a punch accepted by the server (`200`) is
removed from the reader. Without it, it stays on the reader marked as sent.

## Server URL and HTTPS

- `http://` works only to a server in the local network (10.x, 172.16-31.x,
  192.168.x, 169.254.x, IPv6 fc/fd/fe80, or a name that points to such an
  address) and never over an open Wi-Fi.
- For a server on the internet use `https://` with a certificate from a public
  authority (e.g. Let's Encrypt), for example behind nginx or Caddy.
- Keep the URL to plain characters (letters, digits, `- . _ ~ /`). Some HTTP
  servers answer `400` to a URL with a stray `%`, and the reader drops such a punch.
