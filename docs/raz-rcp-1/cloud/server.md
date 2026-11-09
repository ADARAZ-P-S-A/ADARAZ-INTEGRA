# Serwer wzorcowy

Kompletny serwer w Go (tylko biblioteka standardowa, dane w jednym pliku JSON):
[`examples/RAZ-RCP-1/cloud`](https://github.com/ADARAZ-P-S-A/ADARAZ-INTEGRA/tree/master/examples/RAZ-RCP-1/cloud).
Do przeczytania i przeniesienia do własnego systemu.

## Uruchomienie

```bash
cd examples/RAZ-RCP-1/cloud
PUBLIC_URL=https://rcp.example.com ADMIN_TOKEN=$(openssl rand -hex 24) go run .
```

`PUBLIC_URL` to dokładnie adres, który wpiszesz w czytniku (zobacz
[Wymagania](./requirements)). Inne zmienne: `LISTEN` (`:8080`), `DATA_FILE`
(`data.json`), `ORG_NAME`, `TZ_NAME`.

Trasy serwera:

<<< @/../examples/RAZ-RCP-1/cloud/main.go#routes{go}

| Plik | Zawartość |
|---|---|
| `device.go` | API czytnika (`/device/v1`) |
| `admin.go` | API Twojego systemu (`/admin`) |
| `protocol.go` | kody parowania, UID, czas, users.bin, CRC, ETagi |
| `store.go` | dane i zapis do pliku |
| `e2e_test.go` | test: Twój system i czytnik przez prawdziwe HTTP |

## Parowanie czytnika

Kod parowania (8 znaków, ważny 15 minut, jednorazowy):

```bash
curl -X POST -H "Authorization: Bearer $ADMIN_TOKEN" https://rcp.example.com/admin/pairing
```

```json
{"code":"ABCD-EFGH","expires_at":"2026-11-09T10:15:00Z"}
```

W panelu czytnika: **Ustawienia > Integracje > Chmura ADARAZ**:

1. **Własny serwer chmury > Adres serwera**: `https://rcp.example.com`, zapisz.
2. **Kod parowania**: `ABCD-EFGH`, **Połącz**.

Czytnik pokaże „Połączono z organizacją ... jako ...”. Od tej chwili sam wysyła
odbicia, a jego lokalna lista pracowników jest tylko do odczytu: przychodzi
z Twojego serwera.

## Pracownicy i karty

Wysyłasz całą listę naraz, a czytniki pobiorą ją przy najbliższej synchronizacji:

```bash
curl -X PUT -H "Authorization: Bearer $ADMIN_TOKEN" https://rcp.example.com/admin/employees \
  -d '[{"id":"E001","name":"Jan Kowalski","cards":["04a1b2c3"]},
       {"id":"E002","name":"Anna Nowak","cards":["04d4e5f6","0477889900aabb"]}]'
```

Nazwiska nie trafiają do czytnika, tylko numer osoby i jej karty.

## Odbicia

```bash
curl -H "Authorization: Bearer $ADMIN_TOKEN" "https://rcp.example.com/admin/punches?after=0"
```

```json
[{"id":1,"device_id":"...","uid":"04a1b2c3","employee_id":"E001","ts":"2026-11-09T06:58:12Z","received":"2026-11-09T06:58:40Z"}]
```

Pobieraj kolejne z `after=<ostatnie id>`. Nieznane karty: `GET /admin/seen`.
Czytniki: `GET /admin/devices`; `DELETE /admin/devices/{id}` odcina czytnik
(przy następnym połączeniu dostanie `401`).

## Test

```bash
cd examples/RAZ-RCP-1/cloud && go test ./...
```

Test uruchamia prawdziwy serwer HTTP i gra obie strony: Twój system i czytnik
(parowanie, lista pracowników z CRC i `304`, odbicia, duplikat, nieustawiony
zegar, obecność, nieznana karta, złe hasło, usunięcie czytnika, ponowione parowanie).

Uproszczenie serwera wzorcowego: odbicie należy do osoby, która ma kartę w chwili,
gdy odbicie dociera do serwera.
