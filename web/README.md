# Shortr web UI

The admin interface for Shortr: React 19, TypeScript, Vite, Tailwind, and
TanStack Query. In production it is built to `web/dist` and embedded into the
Go binary (see `webdist.go`), so there is nothing to deploy separately.

## Develop

Run the Go backend on `:8080` first (`go run ./cmd/shortr` from the repo
root), then:

```bash
npm install
npm run dev
```

Vite serves the UI with hot reload and proxies `/api` and `/auth` to the
backend. Point it somewhere else by copying `.env.example` to `.env` and
changing `VITE_API_BASE`.

## Scripts

| Command | What it does |
|---|---|
| `npm run dev` | Dev server with HMR |
| `npm run build` | Type-check (`tsc -b`) and produce `dist/` |
| `npm run lint` | Oxlint |

## Layout

```
src/app/         router, providers, layout shell (sidebar, topbar, command palette)
src/components/  shared components; ui/ holds the primitives
src/features/    one folder per area: links, analytics, settings, admin, ...
src/lib/         API client, formatting and small helpers
```

Routes live under `/app`, which is also where the Go server mounts the
built bundle.
