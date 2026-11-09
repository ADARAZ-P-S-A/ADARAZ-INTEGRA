# Parowanie: `POST /claim`

Czytnik wymienia jednorazowy kod parowania na login, hasło i adresy. To jedyna
trasa bez HTTP Basic: kod jest danymi logowania.

## Kod parowania

- 8 znaków z alfabetu `23456789ABCDEFGHJKLMNPQRSTUVWXYZ` (bez 0/O i 1/I), np. `ABCD-EFGH`,
- ważny 15 minut, jednorazowy,
- przyjmuj go w dowolnej wielkości liter, z myślnikiem albo bez, ze spacjami.

## Żądanie

```http
POST /device/v1/claim
Content-Type: application/json

{"code":"ABCD-EFGH","serial":"ef4924d4-4b54-433b-983a-14b62cc8eaeb","firmware":"121"}
```

`serial` to stały identyfikator czytnika, `firmware` to wersja oprogramowania.

## Odpowiedź

```json
{"device":{"id":"...","name":"RAZ-RCP-1 ef4924d4-..."},
 "organization":"Moja firma",
 "credentials":{"url":"https://rcp.example.com/device/v1/punch","login":"rcp-3fa2c1d0e9b8",
                "password":"<43 znaki>","format":3,"interval_min":1},
 "api":"https://rcp.example.com/device/v1"}
```

| Pole | |
|---|---|
| `organization`, `device.name` | pokazywane w panelu czytnika (do 63 bajtów) |
| `credentials.url` | adres odbić: `api` + `/punch` |
| `credentials.login`, `password` | HTTP Basic dla pozostałych tras (każde krótsze niż 64 znaki) |
| `format` | zawsze `3`: UID jako hex |
| `interval_min` | co ile minut czytnik się synchronizuje (1-1440) |
| `api` | baza wszystkich pozostałych tras |

- `url` i `api` muszą spełniać [wymagania adresu](./requirements); cała odpowiedź krótsza niż 2048 bajtów.
- Hasła nie zapisuj jawnie: przechowuj skrót (np. SHA-256 losowych 32 bajtów).

## Błędy

- Zły, zużyty albo wygasły kod: `404 {"code":"bad_pairing_code"}`.
  To jedyna trasa, na której `404` jest poprawną odpowiedzią.
- Czytnik ponawia parowanie do 3 razy, gdy nie dostanie odpowiedzi albo dostanie `5xx`.

## Ponowione parowanie

Kod działa raz. Wyjątek: odpowiedź nie dotarła do czytnika, więc czytnik pyta
jeszcze raz tym samym kodem i tym samym `serial`. Jeśli zrobi to w ciągu
15 minut i zanim użył danych logowania, odpowiedz `200` z tym samym czytnikiem
i loginem, ale **nowym** hasłem. Stare hasło przestaje wtedy działać. Inny
`serial` albo czytnik, który już się logował: `404`.
