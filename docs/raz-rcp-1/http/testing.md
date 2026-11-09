# Test i produkcja

## Test bez czytnika

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

## Przed uruchomieniem produkcyjnym

- `https://` przed serwerem (reverse proxy z publicznym certyfikatem),
- baza danych zamiast pliku, z unikalnym kluczem `(rfidID, timestamp, machineId)`
  i tą samą zasadą: najpierw zapis, potem `200`,
- osobny login i hasło dla każdego czytnika,
- kierunek (wejście/wyjście) liczony po stronie serwera: czytnik go nie wysyła.
  Przykłady łączą odbicia osoby kolejno w pary (1.+2., 3.+4., ...), tak jak panel czytnika.
