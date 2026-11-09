# Odpowiedzi i błędy

Czytnik patrzy przede wszystkim na status odpowiedzi. Błąd z treścią to JSON
`{"error":"<opis>","code":"<stały kod>"}`.

## Co czytnik robi z odpowiedzią

| Odpowiedź | Czytnik |
|---|---|
| `200` | gotowe (odbicie zostaje usunięte z czytnika) |
| `304` | bez zmian (`/users`, `/presence`) |
| `400` | porzuca ten jeden rekord, wysyła resztę |
| `401`, `403` | pokazuje `Chmura odrzucila dane logowania - polacz ponownie`, odbicia czekają w czytniku |
| `404` | traktuje jak zły adres: nic nie porzuca, próbuje później |
| `429`, `5xx`, brak odpowiedzi | próbuje przy następnej synchronizacji |

## Zasady dla Twojego serwera

- **Żadna trasa nie odpowiada `404` na poprawne żądanie** (poza `/claim` ze złym
  kodem). `404` dla nieznanej karty zablokowałby kolejkę czytnika na zawsze.
- **`400` tylko dla rekordu, którego nigdy nie da się zapisać.** Czytnik go porzuci.
- **`401` z nagłówkiem `WWW-Authenticate: Basic realm="rcp"`** przy złym loginie
  lub haśle, także dla czytnika usuniętego w Twoim systemie.
- **JSON bez spacji**, bez kompresji odpowiedzi.
- **Odpowiedź w ciągu 8 sekund.**

## Ponowne połączenie czytnika

Gdy odbierzesz czytnikowi dostęp (nowe hasło albo usunięcie), czytnik zostaje
sparowany, ale każda trasa dostaje `401`. Żeby połączyć go znowu: w panelu
czytnika **Rozłącz**, potem nowy kod parowania. Odbicia czekające w czytniku
przyjdą już z nowymi danymi logowania.

## `GET /hello`

Test połączenia:

```json
{"device":"Wejście główne","organization":"Moja firma","tz":"Europe/Warsaw","epoch":1793404800}
```

`epoch` to czas serwera (sekundy unix), przydatny do sprawdzenia zegara czytnika.
