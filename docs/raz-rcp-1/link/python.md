# Własny serwer w Pythonie

Biblioteka [`adaraz`](https://pypi.org/project/adaraz/) (Python 3.12+) to pełna
implementacja protokołu: parowanie, sesje, szyfrowanie, potwierdzanie odbić,
sterowanie przekaźnikiem. Ty dostajesz zdarzenia.

```bash
pip install "adaraz[aiohttp]"
```

Najmniejszy działający serwer:

<<< @/../examples/RAZ-RCP-1/link/server.py

Uruchom go, a wypisze kod parowania:

```
pairing code: 7QPM-36QP-WNXQ  server URL: http://<this computer>:8765
```

## Parowanie czytnika

W panelu czytnika: **Ustawienia > Integracje > Home Assistant i inne serwery**:

| Pole | Wartość |
|---|---|
| Adres serwera | `http://<IP komputera>:8765` |
| Kod | kod wypisany przez serwer, np. `7QPM-36QP-WNXQ` (ważny 10 minut, jeden czytnik) |

Kliknij **Połącz**. Wielkość liter, myślniki i litery O / I / L (czytane jako
0 / 1 / 1) nie mają znaczenia. Po połączeniu panel pokazuje **Identyfikator
serwera**: możesz go porównać z `server.server_id` po stronie serwera.

## Przekaźnik

Serwer może sterować przekaźnikiem tylko wtedy, gdy administrator czytnika
włączy w tym samym miejscu panelu **Pozwól serwerowi sterować przekaźnikiem**.

```python
result = await server.send_relay(device_id, "open")          # impuls jak po przyłożeniu karty
result = await server.send_relay(device_id, "pulse", ms=1500)  # impuls 1,5 s
result = await server.send_relay(device_id, "on")             # załącz, "off" - wyłącz
if not result.ok:
    print(result.err)  # not_allowed (brak zgody), invalid, busy
```

## Przed uruchomieniem produkcyjnym

- **Zachowaj klucz serwera i sparowane czytniki.** Czytnik zapamiętuje klucz
  publiczny serwera przy parowaniu: nowy klucz po restarcie oznacza ponowne
  parowanie wszystkich czytników. Zaimplementuj `LinkStorage` (lista urządzeń,
  kursor, zapis odbić) zamiast `InMemoryStorage`. Gotowa wersja z plikami:
  [`examples/server.py`](https://github.com/ADARAZ-P-S-A/ADARAZ-HA/blob/master/examples/server.py).
- **Odbicia spóźnione** (`punch.late`, dotarły ponad 5 minut po przyłożeniu)
  zachowaj w historii, ale nie używaj ich do działań na żywo, np. otwierania drzwi.
- **Każde odbicie przychodzi raz:** serwer potwierdza paczkę dopiero, gdy
  `store_punches` zapisze odbicia i kursor. Duplikaty (ta sama karta w tej samej
  sekundzie) filtruje `PunchDeduplicator`.

Pełne API biblioteki: [README biblioteki](https://github.com/ADARAZ-P-S-A/ADARAZ-HA/blob/master/lib/README.md).

## Test bez czytnika {#test}

Biblioteka ma symulowany czytnik (`adaraz.device.LinkDevice`), który robi to
samo co prawdziwy: paruje się kodem, łączy, wysyła odbicia i wykonuje komendy
przekaźnika.

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
