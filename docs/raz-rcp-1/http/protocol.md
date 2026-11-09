# Protokół

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

## Co czytnik robi z odpowiedzią

| Odpowiedź | Czytnik |
|---|---|
| `200` | Odbicie przyjęte: oznaczone jako wysłane albo usunięte. Także przy `"stored":false` w treści. |
| `400` | Odbicie odrzucone: **zdjęte z kolejki i nigdy nie wysłane ponownie**. |
| `401`, `403`, `404`, `5xx`, brak odpowiedzi | Cykl przerwany, odbicie zostaje w czytniku. Kolejna próba po 30 s, potem coraz rzadziej. |

## Zasady dla Twojego serwera

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
