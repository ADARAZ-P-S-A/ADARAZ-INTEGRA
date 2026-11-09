# Wysyłanie odbić (HTTP)

Najprostsza integracja: czytnik co ustawiony interwał wysyła nowe odbicia na
adres Twojego serwera, każde w osobnym żądaniu `POST`. Twój serwer zapisuje
odbicie i odpowiada `200`. Nic więcej nie jest potrzebne.

```
czytnik ── POST rfidID, timestamp, machineId ──▶ Twój serwer
        ◀────────── 200 {"stored":true} ─────────
```

## Spis

| Strona | Co jest na niej |
|---|---|
| [Serwer](./server) | kompletny serwer w Pythonie, Node.js, Go i PHP; pełne przykłady w 6 językach |
| [Ustawienia czytnika](./reader) | co wpisać w panelu czytnika, zasady adresu i HTTPS |
| [Protokół](./protocol) | żądanie, pola, co czytnik robi z odpowiedzią, zasady dla serwera |
| [Test i produkcja](./testing) | symulator czytnika, lista kontrolna przed uruchomieniem |

## W skrócie

1. Uruchom [serwer](./server), który na `POST` zapisuje odbicie i odpowiada `200`.
2. W czytniku wpisz jego adres, login i hasło ([ustawienia czytnika](./reader)).
3. Sprawdź go [symulatorem czytnika](./testing).
