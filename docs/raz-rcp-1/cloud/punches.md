# Odbicia i nieznane karty

## `POST /punch`

Czytnik wysyła każde odbicie w osobnym żądaniu, do 32 w jednym cyklu, jednym
połączeniem keep-alive:

```http
POST /device/v1/punch
Authorization: Basic ...
Content-Type: application/x-www-form-urlencoded

rfidID=04a1b2c3&timestamp=2026-11-09+06%3A58%3A12&machineId=RAZ-RCP-1
```

| Pole | |
|---|---|
| `rfidID` | UID karty w hex, 4-7 bajtów (wielkość liter, `:`, `-` i spacje bez znaczenia) |
| `timestamp` | `RRRR-MM-DD GG:MM:SS` w UTC |
| `machineId` | pomiń: czytnik rozpoznajesz po loginie |

### Odpowiedzi

| Odpowiedź | Kiedy |
|---|---|
| `200 {"stored":true,"duplicate":false}` | zapisane |
| `200 {"stored":false,"duplicate":true}` | to samo odbicie drugi raz (klucz: czytnik, karta, czas) |
| `200 {"stored":false,"reason":"clock"}` | czas przed 2020 albo ponad dobę w przód: zegar czytnika był nieustawiony |
| `400 {"code":"bad_uid"}` | `rfidID` nie jest UID w hex |
| `400 {"code":"bad_time"}` | nie da się odczytać `timestamp` |

- **Odpowiadaj `200` dopiero po zapisie.** Po `200` czytnik usuwa odbicie.
- **Nieznana karta to też `200`.** Odbicie jest prawdziwe; kartę przypiszesz później.
  `404` albo `5xx` zablokowałyby kolejkę czytnika.
- **Duplikat to `200`.** Gdy zginie Twoja odpowiedź, czytnik wyśle odbicie jeszcze raz.
- **JSON bez spacji.** Czytnik szuka w odpowiedzi dokładnie `"stored":false`.
- Odbicie należy do osoby, która miała kartę w chwili odbicia (`timestamp`).

## `POST /seen`

Czytnik zgłasza od razu kartę, której nie ma na jego liście: także taką, której
odmówił wejścia, więc nie wysłał dla niej odbicia. Dzięki temu można przyłożyć
nową kartę i przypisać ją w Twoim systemie.

```http
POST /device/v1/seen
Authorization: Basic ...
Content-Type: application/x-www-form-urlencoded

rfidID=04c1c2c3&timestamp=2026-11-09+07%3A01%3A00&denied=1
```

`denied=1`: czytnik odmówił; `0`: przepuścił i zapisał odbicie.

| Odpowiedź | Kiedy |
|---|---|
| `200 {"stored":true}` | zapisane |
| `200 {"stored":false,"reason":"clock"}` | nieustawiony zegar czytnika |
| `400` | nie da się odczytać `rfidID` albo `timestamp` |

Po przypisaniu karty w Twoim systemie trafia ona do następnej
[listy pracowników](./users).
