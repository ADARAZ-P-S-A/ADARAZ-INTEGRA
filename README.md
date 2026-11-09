# ADARAZ Integra - integrator portal

**https://integra.adaraz.com**

How to connect ADARAZ devices to your own system: the portal and all its example
code. Built with [VitePress](https://vitepress.dev) and published on GitHub
Pages, in Polish (default) and English.

| Path | Content |
|---|---|
| `docs/` | portal content (Markdown): Polish at the root, English in `docs/en/` |
| `docs/shared/` | the ADARAZ Link specification (a copy from [ADARAZ-HA](https://github.com/ADARAZ-P-S-A/ADARAZ-HA)) |
| `docs/.vitepress/` | configuration, navigation and colours |
| `examples/RAZ-RCP-1/http/` | punch push (HTTP): servers in six languages, minimal servers, reader simulator, OpenAPI |
| `examples/RAZ-RCP-1/cloud/` | own cloud server: reference implementation of the ADARAZ Cloud device API, Go |
| `examples/RAZ-RCP-1/link/` | ADARAZ Link: the smallest server with the `adaraz` Python library |

The portal pages include code straight from `examples/`, so what is shown is
what is tested.

## Local development

```bash
npm ci
```

```bash
npm run dev
```

The site runs at `http://localhost:5173`. `npm run build` builds it into
`docs/.vitepress/dist`.

## Tests

Punch push: every example against the reader simulator (missing language
runtimes are skipped):

```bash
examples/RAZ-RCP-1/http/symulator/test_wszystkie.sh
```

Own cloud server: end-to-end test, your system and a reader over real HTTP:

```bash
cd examples/RAZ-RCP-1/cloud && go test ./...
```

CI runs both, plus an OpenAPI check (`.github/workflows/test.yml`).

## Publishing

Every push to `master` builds and publishes the portal (`.github/workflows/pages.yml`).
One-time setup in the repository: **Settings > Pages > Build and deployment >
Source: GitHub Actions**, custom domain `integra.adaraz.com`, **Enforce HTTPS**.

## Adding a device

1. Portal pages in `docs/<device>/` and `docs/en/<device>/`, starting with `index.md`.
2. A card on the home page (`docs/index.md`, `docs/en/index.md`).
3. A sidebar entry in `docs/.vitepress/config.mts`.
4. Example code in `examples/<device>/`.

## License

Apache License 2.0. The ADARAZ Link specification in `docs/shared/` is MIT,
as in ADARAZ-HA.
