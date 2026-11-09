# Lista pracowników: `GET /users`

Czytnik pobiera z Twojego serwera listę kart, które zna (plik users.bin).
Po sparowaniu to jedyne źródło tej listy: w panelu czytnika jest tylko do odczytu.

## Żądanie

```http
GET /device/v1/users
Authorization: Basic ...
If-None-Match: "e3b0c44298fc1c149afbf4c8996fb924"
```

`If-None-Match` to skrót pliku, który czytnik ma: pierwsze 16 bajtów SHA-256
jego zawartości, hex małymi literami. Pusty plik:
`e3b0c44298fc1c149afbf4c8996fb924`. Czytnik może dopisać po przecinku drugą
wartość: skrót listy, którą wcześniej odrzucił (np. za długą), żeby nie pobierać
jej co minutę.

## Odpowiedź

- skrót Twojej listy równy któremukolwiek z `If-None-Match`: `304` bez treści,
- inaczej `200`, `Content-Type: application/octet-stream`, treść = lista.

W obu przypadkach nagłówki `ETag: "<skrót listy>"` i `X-RCP-Users: <liczba rekordów>`.
Pusta lista to `200` z pustą treścią.

## Format

Rekordy po 48 bajtów, jeden na kartę:

| Bajty | |
|---|---|
| `[0]` | `0x00` |
| `[1]` | długość UID, 4-7 |
| `[2..8]` | UID, dopełniony zerami do 7 bajtów |
| `[9..45]` | numer osoby (`emp_no`) jako cyfry ASCII (1-20), dalej bajty zerowe |
| `[46..47]` | CRC-16/CCITT-FALSE z bajtów `[0..45]`, little-endian |

CRC-16/CCITT-FALSE: wielomian `0x1021`, start `0xFFFF`, bez odbicia, bez końcowego XOR.
Dla `"123456789"` wynosi `0x29B1`.

- Kolejność: po `emp_no` jako liczbie, potem po UID w hex. Ta sama lista daje zawsze
  te same bajty, więc i ten sam skrót.
- `emp_no` to stały numer osoby. Dwie karty jednej osoby to dwa rekordy z tym samym
  `emp_no`: po nim czytnik łączy wejścia i wyjścia.
- Bez nazwisk: czytnik nie potrzebuje danych osobowych.
- Najwyżej 2048 rekordów (98 304 bajty). Dłuższą listę czytnik odrzuca i zostaje przy starej.

## Jak czytnik sprawdza listę

Jeden zły rekord odrzuca całą listę (czytnik zostaje przy poprzedniej):
rozmiar niepodzielny przez 48, `X-RCP-Users` niezgodny z liczbą rekordów,
zła suma CRC albo długość UID spoza 4-7.

Czytnik pyta najwyżej co 50 sekund oraz od razu po sparowaniu i po uruchomieniu.
