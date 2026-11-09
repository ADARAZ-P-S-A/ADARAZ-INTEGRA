# Serwer

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
