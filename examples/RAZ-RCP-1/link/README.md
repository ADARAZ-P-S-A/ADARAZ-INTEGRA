# RAZ-RCP-1 - ADARAZ Link example

The smallest ADARAZ Link server, built with the [`adaraz`](https://pypi.org/project/adaraz/)
Python library (Python 3.12+). Full description on the portal:
https://integra.adaraz.com/en/raz-rcp-1/link

```bash
pip install "adaraz[aiohttp]"
```

```bash
python3 server.py
```

It prints a pairing code. In the reader's panel: **Settings > Integrations >
Home Assistant and other servers**, enter `http://<this computer>:8765` and the code.

The key and the pairing live in memory (demo). For a server that keeps them, see
[`examples/server.py`](https://github.com/ADARAZ-P-S-A/ADARAZ-HA/blob/master/examples/server.py)
in ADARAZ-HA.
