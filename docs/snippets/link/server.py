# Minimal ADARAZ Link server (Python 3.12+): pip install "adaraz[aiohttp]"
# Reader: Settings > Integrations > Home Assistant and other servers,
# server URL http://<this computer>:8765 and the code printed below.
import asyncio

from aiohttp import web
from adaraz import (DeviceConnected, InMemoryStorage, LinkServer, PunchReceived,
                    generate_private_key)


async def main() -> None:
    # Demo only: the key and the pairing live in memory. In production keep both
    # (see examples/server.py in ADARAZ-HA): the reader pins the server key at pairing.
    server = LinkServer(generate_private_key(), InMemoryStorage(), server="My app", name="Office")

    def on_event(event) -> None:
        if isinstance(event, DeviceConnected):
            print("reader online:", event.device.device_id, event.hello.name, flush=True)
        elif isinstance(event, PunchReceived):
            p = event.punch
            late = " (late)" if p.late else ""
            print(f"punch: card {p.uid} at {p.ts} from {p.device_id}{late}", flush=True)

    server.subscribe(on_event)

    app = web.Application()
    app.router.add_get("/api/adaraz/ws", server.handle_aiohttp)  # the reader connects here
    runner = web.AppRunner(app)
    await runner.setup()
    await web.TCPSite(runner, "0.0.0.0", 8765).start()

    window = server.open_pairing()  # one reader, 10 minutes
    print(f"pairing code: {window.code}  server URL: http://<this computer>:8765", flush=True)
    device = await window
    print("paired:", device.device_id, flush=True)

    # Optional: open the door. Works only if the reader's admin allowed it in the
    # reader panel ("Pozwól serwerowi sterować przekaźnikiem").
    # result = await server.send_relay(device.device_id, "open")

    await asyncio.Event().wait()


asyncio.run(main())
