# The server

A complete server in four languages: it checks the login and password, writes
the punch to a file and only then answers `200`, and recognises duplicates and
punches made with an unset clock. Each is a few dozen lines with no libraries.

::: code-group

<<< @/../examples/RAZ-RCP-1/http/minimal/server.py [Python]

<<< @/../examples/RAZ-RCP-1/http/minimal/server.js [Node.js]

<<< @/../examples/RAZ-RCP-1/http/minimal/main.go [Go]

<<< @/../examples/RAZ-RCP-1/http/minimal/index.php [PHP]

:::

Run it (with the same password as in the reader):

::: code-group

```bash [Python]
RCP_HASLO=secret python3 server.py
```

```bash [Node.js]
RCP_HASLO=secret node server.js
```

```bash [Go]
RCP_HASLO=secret go run main.go
```

```bash [PHP]
RCP_HASLO=secret php -S 0.0.0.0:8080 index.php
```

:::

The server listens on port 8080 and appends punches to `odbicia.tsv`
(`epoch<TAB>rfidID<TAB>machineId`).

Full examples in six languages (Python, Go, Node.js, Java, C#, PHP), with a
working time report and an employee list, are in
[`examples/RAZ-RCP-1/http`](https://github.com/ADARAZ-P-S-A/ADARAZ-INTEGRA/tree/master/examples/RAZ-RCP-1/http).
