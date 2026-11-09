# ADARAZ Integra - integrator portal

How to connect ADARAZ devices to your own system. Built with
[VitePress](https://vitepress.dev) and published on GitHub Pages, in Polish
(default) and English.

| Path | Content |
|---|---|
| `docs/` | site content (Markdown): Polish at the root, English in `docs/en/` |
| `docs/snippets/` | example code shown on the pages (included from these files, not copied) |
| `docs/shared/` | the ADARAZ Link specification (a copy from [ADARAZ-HA](https://github.com/ADARAZ-P-S-A/ADARAZ-HA)) |
| `docs/.vitepress/` | configuration, navigation and colours |

## Local development

```bash
npm ci
```

```bash
npm run dev
```

The site runs at `http://localhost:5173`. `npm run build` builds it into
`docs/.vitepress/dist`.

## Publishing

Every push to `main` builds and publishes the site (`.github/workflows/pages.yml`).
One-time setup in the repository: **Settings > Pages > Build and deployment >
Source: GitHub Actions**.

## Adding a device

1. A `docs/<device>/` directory (and `docs/en/<device>/`) with an `index.md`.
2. A card on the home page (`docs/index.md`, `docs/en/index.md`).
3. A sidebar entry in `docs/.vitepress/config.mts`.

## Example code

The servers in `docs/snippets/http/` follow the reader's protocol and are
checked with the reader simulator from
[RAZ-RCP-1-integrations](https://github.com/ADARAZ-P-S-A/RAZ-RCP-1-integrations):

```bash
RCP_HASLO=secret python3 docs/snippets/http/server.py
```

```bash
python3 symulator/symulator_czytnika.py http://127.0.0.1:8080/odbicia --haslo secret --test
```

`RCP_HASLO` and `odbicia.tsv` keep the names used by the full examples in
RAZ-RCP-1-integrations, so the two stay interchangeable.

## License

Apache License 2.0. The ADARAZ Link specification in `docs/shared/` is MIT,
as in ADARAZ-HA.
