# ADARAZ Link

Stałe, szyfrowane połączenie czytnika z Twoim serwerem. Czytnik sam łączy się
z serwerem przez WebSocket i trzyma połączenie otwarte:

- odbicia docierają zaraz po przyłożeniu karty, a te z czasu bez sieci są
  dosyłane później i oznaczone jako spóźnione,
- serwer widzi stan czytnika (połączony, czy czytnik kart działa, stan przekaźnika),
- serwer może sterować przekaźnikiem, np. otwierać drzwi, jeśli administrator
  czytnika na to pozwoli.

```
czytnik ══ WebSocket + Noise (łączy się sam) ══ Twój serwer
   odbicia, stan ─────────────────────────────▶
                 ◀───────────────────────────── potwierdzenia,
                                                przekaźnik
```

Każde połączenie to sesja [Noise](https://noiseprotocol.org/) ze wzajemnym
uwierzytelnieniem kluczami wymienionymi przy parowaniu, więc zwykłe `ws://`
w sieci lokalnej jest bezpieczne. Dla serwera w internecie działa też `wss://`.

## Trzy drogi

| | Dla kogo |
|---|---|
| [Home Assistant](#home-assistant) | gotowa integracja, bez pisania kodu |
| [Własny serwer w Pythonie](#python) | biblioteka `adaraz` robi cały protokół, Ty piszesz tylko obsługę odbić |
| [Inny język](#inny-jezyk) | własna implementacja według specyfikacji |

## Home Assistant {#home-assistant}

Integracja ADARAZ dla Home Assistant: odbicia jako zdarzenia, czujnik
połączenia, przycisk i przełącznik przekaźnika. Instalacja przez HACS,
opis w repozytorium [ADARAZ-HA](https://github.com/ADARAZ-P-S-A/ADARAZ-HA).

## Własny serwer w Pythonie {#python}

Biblioteka [`adaraz`](https://pypi.org/project/adaraz/) (Python 3.12+) to pełna
implementacja protokołu: parowanie, sesje, szyfrowanie, potwierdzanie odbić,
sterowanie przekaźnikiem. Ty dostajesz zdarzenia.

```bash
pip install "adaraz[aiohttp]"
```

Najmniejszy działający serwer:

<<< @/snippets/link/server.py

Uruchom go, a wypisze kod parowania:

```
pairing code: 7QPM-36QP-WNXQ  server URL: http://<this computer>:8765
```

### Parowanie czytnika

W panelu czytnika: **Ustawienia > Integracje > Home Assistant i inne serwery**:

| Pole | Wartość |
|---|---|
| Adres serwera | `http://<IP komputera>:8765` |
| Kod | kod wypisany przez serwer, np. `7QPM-36QP-WNXQ` (ważny 10 minut, jeden czytnik) |

Kliknij **Połącz**. Wielkość liter, myślniki i litery O / I / L (czytane jako
0 / 1 / 1) nie mają znaczenia. Po połączeniu panel pokazuje **Identyfikator
serwera**: możesz go porównać z `server.server_id` po stronie serwera.

### Przekaźnik

Serwer może sterować przekaźnikiem tylko wtedy, gdy administrator czytnika
włączy w tym samym miejscu panelu **Pozwól serwerowi sterować przekaźnikiem**.

```python
result = await server.send_relay(device_id, "open")          # impuls jak po przyłożeniu karty
result = await server.send_relay(device_id, "pulse", ms=1500)  # impuls 1,5 s
result = await server.send_relay(device_id, "on")             # załącz, "off" - wyłącz
if not result.ok:
    print(result.err)  # not_allowed (brak zgody), invalid, busy
```

### Przed uruchomieniem produkcyjnym

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

### Test bez czytnika

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

## Inny język {#inny-jezyk}

Protokół jest w pełni opisany w [specyfikacji ADARAZ Link](./link-protocol).
W skrócie:

1. **Transport:** WebSocket pod `/api/adaraz/ws`, podprotokół `adaraz.v1`, tylko
   wiadomości binarne, jedna wiadomość WebSocket = jedna wiadomość Noise (do 4096 B).
2. **Parowanie:** `Noise_XXpsk3_25519_AESGCM_SHA256`, PSK wyliczany z 12-znakowego
   kodu (HKDF-SHA256). Serwer zapamiętuje klucz publiczny czytnika, czytnik klucz serwera.
3. **Sesja:** `Noise_IK_25519_AESGCM_SHA256`. Czytnik wysyła `hello`, serwer
   odpowiada `welcome` z kursorem, od którego czytnik ma wysyłać odbicia.
4. **Odbicia:** wiadomości `punches` (do 100 pozycji `[indeks, uid_hex, unix_ts]`),
   serwer odpowiada `ack` dopiero po trwałym zapisie.
5. **Przekaźnik:** `relay` -> `relay_result`, tylko za zgodą administratora czytnika.

Do sprawdzenia własnej implementacji użyj symulowanego czytnika z biblioteki
`adaraz` (wyżej): to niezależna, przetestowana implementacja strony czytnika.
