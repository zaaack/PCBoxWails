# AGENTS.md

## Project Overview

PCBox is a Wails v2 desktop app — Go backend + React/TypeScript frontend. It runs a WebSocket server for TV-K mobile app communication and a local proxy server for CORS/M3U8 rewriting.

## Build Commands

| Task | Command |
|------|---------|
| Dev server | `wails dev` |
| Production build | `wails build` |
| Frontend only (dev) | `npm run dev` (in `frontend/`) |
| Frontend build check | `npm run build` (in `frontend/`) — runs `tsc && vite build` |

No separate Go build needed — `wails dev`/`wails build` handles everything.

## Architecture

```
Go backend (main.go, app.go, ws-server.go, proxy-server.go)
  ├── WebSocket server (default port 9898)
  ├── Proxy server (random port, 127.0.0.1)
  └── Exposes methods to frontend via Wails IPC

Frontend (frontend/src/)
  ├── App.tsx — root, initializes WS server on mount
  ├── store/index.ts — Zustand store, all state + WS message dispatch
  ├── lib/api.ts — IPC wrapper around window.go.main.App.*
  ├── components/ — React views (Home, Search, Player, etc.)
  └── wailsjs/go/main/ — AUTO-GENERATED Go bindings (do not edit)
```

## Key Gotchas

- **wailsjs bindings are auto-generated.** Do not edit files in `frontend/wailsjs/`. Regenerate with `wails generate module`.
- **Go embed requires `frontend/dist/` to exist.** The `//go:embed all:frontend/dist` directive in `main.go` embeds built frontend assets. Run frontend build before `go build` if `dist/` is missing.
- **Message protocol uses numeric codes.** See `MessageCodes` in `store/index.ts` for the full list (100=REGISTER, 201=GET_SOURCES, 203=GET_HOME, etc.).
- **Topic-based request/response.** Frontend generates a `topicId`, sends it with the request, and registers a callback. Response arrives via `ws-response` event with matching `topicId`.
- **Proxy server rewrites M3U8 playlists.** All sub-URLs in `.m3u8` responses are rewritten to go through the local proxy. This is why `proxy-server.go` has special M3U8 handling.
- **Single instance lock.** App uses `com.pcbox.app` unique ID — only one instance allowed.
- **WebSocket port hardcoded to 9898.** Set in `App.tsx:47` during init. TV-K mobile app connects to this port.
- **Proxy server uses random port.** Binds to `127.0.0.1:0` (OS-assigned) in `proxy-server.go:36`. Port returned via `CreateProxySession()` for frontend use.

## Frontend Conventions

- Path alias: `@/` maps to `frontend/src/` (vite + tsconfig)
- State: single Zustand store at `store/index.ts`
- No CSS modules — plain CSS in `styles/`
- Video player: video.js with hotkeys plugin

## Go Conventions

- All Go files are in package `main` at project root
- Wails lifecycle: `startup()` creates proxy server, `shutdown()` stops both servers
- WS server is started on-demand from frontend via `StartWsServer(port)`
