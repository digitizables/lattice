# Operator web UI

Package: `client/ui/web` (Svelte 5 + Vite + TypeScript).

## Surfaces

- **Sessions** — searchable, paginated fleet table; auto-refresh ~15s; full-bleed table chrome
- **Console** — multi-tab terminal emulator, split panes (context menu), native shell lines, ANSI via `ansi_up`, input highlight via `highlight.js`, live streaming output, selectable scrollback
- **Listeners** — live `ListListeners`; **Reload extensions** button
- **Documentation** — Operators / Developers markdown bundles (`@docs` Vite alias)
- **Settings** — path display preference; connection hints; agent update + elevate module catalogs

## Dev vs Compose

| Mode | URL | API |
|---|---|---|
| Vite | http://127.0.0.1:5173 | Direct to `:8080` (CORS allowlist) |
| Compose `web` | http://127.0.0.1:3000 | nginx proxies Connect paths |

```text
cd client/ui/web
npm install
npm run dev
```

## UI conventions

Shared tokens in `src/app.css`: spacing `--space-1…6`, radii `--radius-sm|md|lg`, control heights, page pad. Prefer flush panels over floating cards; Sessions/Listeners tables are full-bleed.
