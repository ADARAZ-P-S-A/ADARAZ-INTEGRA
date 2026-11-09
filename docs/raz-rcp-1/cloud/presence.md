# Obecność: `GET /presence`

Osoba może wejść jednym czytnikiem, a wyjść innym, a każdy czytnik zna tylko
swoje odbicia. Dlatego po wysłaniu odbić czytnik pyta, kto jest w środku, i na
tej podstawie ustala kierunek następnego odbicia osoby.

## Żądanie

```http
GET /device/v1/presence
Authorization: Basic ...
If-None-Match: "<ETag ostatnio zastosowanej odpowiedzi>"
X-RCP-Users: "<skrót users.bin czytnika>"
```

`If-None-Match` jest pusty po uruchomieniu czytnika. `X-RCP-Users` służy tylko do logów.

## Odpowiedź

Jeden bit na rekord [listy pracowników](./users), w tej samej kolejności:

- rekord `i` to bajt `i/8`, bit `i%8`, bit 0 najmłodszy,
- `1` = właściciel karty jest w środku,
- długość `ceil(N/8)` bajtów; pusta lista = pusta treść.

| Nagłówek | |
|---|---|
| `X-RCP-Users-ETag` | skrót listy, której dotyczą bity |
| `ETag` | pierwsze 16 bajtów SHA-256 z (skrót listy jako 32 znaki hex, potem bity), hex |

- zgodny `If-None-Match`: `304` z tymi samymi nagłówkami,
- inaczej `200`, `Content-Type: application/octet-stream`, treść = bity.

## Zasady

- **W środku** = nieparzysta liczba odbić osoby (z każdego czytnika, bez usuniętych).
  Obie karty jednej osoby mają ten sam bit.
- **Odbicie przyjęte przed chwilą musi już być w odpowiedzi.** Czytnik pyta zaraz
  po wysłaniu odbić i ufa odpowiedzi.
- Czytnik stosuje bity tylko wtedy, gdy `X-RCP-Users-ETag` równa się skrótowi jego
  listy. Inaczej czeka na następną synchronizację.
- Czytnik pyta najwyżej co 20 sekund i nie pyta wcale, gdy ma na liście ponad 256 kart.
