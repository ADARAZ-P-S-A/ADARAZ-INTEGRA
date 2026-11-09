# Wysyłanie odbić (HTTP)

Najprostsza integracja: czytnik co ustawiony interwał wysyła nowe odbicia na
adres Twojego serwera, każde w osobnym żądaniu `POST`. Twój serwer zapisuje
odbicie i odpowiada `200`. Nic więcej nie jest potrzebne.

```
czytnik ── POST rfidID, timestamp, machineId ──▶ Twój serwer
        ◀────────── 200 {"stored":true} ─────────
```

## 1. Serwer

Poniżej kompletny serwer w czterech językach: sprawdza login i hasło, zapisuje
odbicie do pliku i dopiero wtedy odpowiada `200`, rozpoznaje duplikaty i odbicia
z nieustawionym zegarem. Każdy ma kilkadziesiąt linii i nie wymaga żadnych bibliotek.

::: code-group

<<< @/../examples/RAZ-RCP-1/http/minimal/server.py [Python]

<<< @/../examples/RAZ-RCP-1/http/minimal/server.js [Node.js]

<<< @/../examples/RAZ-RCP-1/http/minimal/main.go [Go]

<<< @/../examples/RAZ-RCP-1/http/minimal/index.php [PHP]

:::

Uruchomienie (hasło ustawiasz takie samo jak w czytniku):

::: code-group

```bash [Python]
RCP_HASLO=tajne python3 server.py
```

```bash [Node.js]
RCP_HASLO=tajne node server.js
```

```bash [Go]
RCP_HASLO=tajne go run main.go
```

```bash [PHP]
RCP_HASLO=tajne php -S 0.0.0.0:8080 index.php
```

:::

Serwer słucha na porcie 8080 i dopisuje odbicia do `odbicia.tsv`
(`epoch<TAB>rfidID<TAB>machineId`).

Rozbudowane przykłady w sześciu językach (Python, Go, Node.js, Java, C#, PHP),
z raportem czasu pracy i listą pracowników, są w katalogu
[`examples/RAZ-RCP-1/http`](https://github.com/ADARAZ-P-S-A/ADARAZ-INTEGRA/tree/master/examples/RAZ-RCP-1/http).

## 2. Ustawienia czytnika

W panelu czytnika: **Ustawienia > Integracje > Synchronizacja**:

| Pole | Wartość |
|---|---|
| Wysyłaj odbicia na serwer | włączone |
| Format | `http3 - UID jako hex` |
| Adres serwera (URL) | `http://<IP komputera>:8080/odbicia` |
| Login / Hasło | `rcp` / wartość `RCP_HASLO` |
| ID urządzenia (machineId) | dowolna nazwa, np. `Brama` |
| Interwał [min] | 1 do testów, fabrycznie 15 |

Opcja „Usuwaj odbicia po wysłaniu” sprawia, że odbicie przyjęte przez serwer
(`200`) znika z czytnika. Bez niej zostaje w czytniku oznaczone jako wysłane.

## 3. Test bez czytnika

[Symulator czytnika](https://github.com/ADARAZ-P-S-A/ADARAZ-INTEGRA/blob/master/examples/RAZ-RCP-1/http/symulator/symulator_czytnika.py)
(Python 3.9+, bez zależności) wysyła odbicia dokładnie jak czytnik i sprawdza
odpowiedzi Twojego serwera: złe hasło, nowe odbicie, duplikat, nieustawiony
zegar, błędne dane i znaki specjalne w `machineId`.

```bash
python3 symulator_czytnika.py http://127.0.0.1:8080/odbicia --haslo tajne --test
```

```bash
python3 symulator_czytnika.py http://127.0.0.1:8080/odbicia --haslo tajne --karta 04a1b2c3
```

Tryb `--test` sprawdza na końcu także stronę `GET /raport` z rozbudowanych
przykładów. Jeśli Twój serwer jej nie ma, pomiń trzy ostatnie sprawdzenia.

## Protokół

Czytnik wysyła **każde odbicie w osobnym żądaniu `POST`**, najwyżej 32 odbicia
w jednym cyklu, jednym połączeniem keep-alive:

```http
POST /odbicia HTTP/1.1
Authorization: Basic cmNwOnRham5l
Content-Type: application/x-www-form-urlencoded
User-Agent: RAZ-RCP/8

rfidID=04a1b2c3d4e5f6&timestamp=2026-10-28+06%3A58%3A12&machineId=Brama
```

| Pole | Treść |
|---|---|
| `rfidID` | Numer karty. `http3`: UID w hex małymi literami (8 albo 14 znaków). `http2`: ten sam UID jako liczba dziesiętna. Skryba: numer liczony jak w starej Skrybie. |
| `timestamp` | `RRRR-MM-DD GG:MM:SS`, bez strefy: UTC w `http2`/`http3`, czas polski w formatach Skryba. Rok przed 2026 oznacza nieustawiony zegar czytnika. |
| `machineId` | ID czytnika z jego ustawień, może być puste. |

Pełny opis w OpenAPI:
[`openapi.yaml`](https://github.com/ADARAZ-P-S-A/ADARAZ-INTEGRA/blob/master/examples/RAZ-RCP-1/http/openapi.yaml).

### Co czytnik robi z odpowiedzią

| Odpowiedź | Czytnik |
|---|---|
| `200` | Odbicie przyjęte: oznaczone jako wysłane albo usunięte. Także przy `"stored":false` w treści. |
| `400` | Odbicie odrzucone: **zdjęte z kolejki i nigdy nie wysłane ponownie**. |
| `401`, `403`, `404`, `5xx`, brak odpowiedzi | Cykl przerwany, odbicie zostaje w czytniku. Kolejna próba po 30 s, potem coraz rzadziej. |

### Zasady dla Twojego serwera

1. **Odpowiadaj `200` dopiero, gdy odbicie jest bezpiecznie zapisane.** Czytnik
   może je zaraz usunąć.
2. **`400` tylko dla żądania, którego nigdy nie da się zapisać.** W razie
   wątpliwości odpowiedz `5xx`: czytnik spróbuje później.
3. **To samo odbicie może przyjść dwa razy**, np. gdy zginęła Twoja odpowiedź.
   Odpowiedz `200` (`{"stored":false,"reason":"duplicate"}`), inaczej czytnik
   będzie je wysyłał dalej. Kluczem jest `(rfidID, timestamp, machineId)`.
4. **Odbicie z rokiem przed 2026 powstało przy nieustawionym zegarze.**
   Odpowiedź `{"stored":false,"reason":"clock"}` czytnik zapisze w swoim logu.
5. **JSON bez spacji.** Czytnik szuka w odpowiedzi dokładnie `"stored":false`.

## Adres serwera i HTTPS

- `http://` działa tylko do serwera w sieci lokalnej (10.x, 172.16-31.x,
  192.168.x, 169.254.x, IPv6 fc/fd/fe80 albo nazwa wskazująca taki adres)
  i nigdy przez otwartą sieć Wi-Fi.
- Do serwera w internecie użyj `https://` z certyfikatem publicznego urzędu
  certyfikacji (np. Let's Encrypt), np. za nginx albo Caddy.
- W adresie używaj zwykłych znaków (litery, cyfry, `- . _ ~ /`). Niektóre serwery
  HTTP odpowiadają `400` na adres z luźnym `%`, a takie odbicie czytnik porzuca.

## Przed uruchomieniem produkcyjnym

- `https://` przed serwerem (reverse proxy z publicznym certyfikatem),
- baza danych zamiast pliku, z unikalnym kluczem `(rfidID, timestamp, machineId)`
  i tą samą zasadą: najpierw zapis, potem `200`,
- osobny login i hasło dla każdego czytnika,
- kierunek (wejście/wyjście) liczony po stronie serwera: czytnik go nie wysyła.
  Przykłady łączą odbicia osoby kolejno w pary (1.+2., 3.+4., ...), tak jak panel czytnika.
