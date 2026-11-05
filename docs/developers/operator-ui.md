# Operator web UI

Package: `client/ui/web` (Svelte 5 + Vite + TypeScript).

## Surfaces

- **Sessions** — searchable, paginated fleet table; notes (`SetSessionNote`); sleep/jitter (`SetAgentSleep`); feature chips; auto-refresh ~15s; full-bleed table chrome
- **Console** — multi-tab terminal emulator, split panes (context menu), native shell lines, ANSI via `ansi_up`, input highlight via `highlight.js`, live streaming output, selectable scrollback
- **Listeners** — live `ListListeners`; **Reload extensions** button
- **Documentation** — Operators / Developers markdown bundles (`@docs` Vite alias)
- **Settings** — account logout; path display; connection hints; generate agent (`GenerateAgent` download); loot list/download/delete; agent update + elevate module catalogs

## Auth

- Operator API requires `Authorization: Bearer <token>` unless `LATTICE_AUTH_DISABLED=true`.
- On HTTP 401 the SPA shows a login form; successful `Login` stores the token in `localStorage` under `lattice.operator.token`.
- Default bootstrap operator: **admin** / **lattice-change-me** (`LATTICE_BOOTSTRAP_OPERATOR` / `LATTICE_BOOTSTRAP_PASSWORD`).
- Settings → Account → Log out calls `Logout` and clears the stored token.

## Dev vs Compose

- **Vite:** http://127.0.0.1:5173 — API direct to `:8080` (CORS allowlist)
- **Compose `web`:** http://127.0.0.1:3000 — nginx proxies Connect paths

```text
cd client/ui/web
npm install
npm run dev
```

## UI conventions

Shared tokens in `src/app.css`: spacing `--space-1…6`, radii `--radius-sm|md|lg`, control heights, page pad. Prefer flush panels over floating cards; Sessions/Listeners tables are full-bleed.
