# AGENTS.md

## Project

termchat — minimal anonymous terminal chatrooms. A **single Go module** (`module termchat`, no go.work) with three packages:

- `cli/` — Bubble Tea TUI client, argument parsing, LAN host mode (embeds the WebSocket server in-process), LAN discovery
- `server/` — WebSocket server, room management, host succession, room discovery, bootstrap one-liner install scripts, `/healthz`
- `shared/` — protocol types (`Message`, `UserInfo`, `RoomInfo`), room code validation/generation, discovery constants

There is no separate API server: the server binary serves the WebSocket protocol, `/discover`, and the `curl | bash` bootstrap flow (scripts embedded via `go:embed`, `/bin/{binary}` redirects to GitHub Releases with a whitelist).

## Commands

```bash
just check        # full CI gate: tidy-check, fmt-check, vet, build, test -race
go test -race ./...   # all tests must pass under the race detector
just cross        # cross-compile all 8 release platforms
just server       # run WebSocket server locally (port 8080)
make build        # official CLI build with version ldflags -> dist/
make install      # packaging (binary, man page, license)
```

## Conventions

- `gofmt` + `go vet` clean; `go mod tidy` before committing. All of this is enforced in CI.
- **Server concurrency** (non-negotiable):
  - `rooms` map only under `roomsMutex` (read AND write).
  - Mutable client fields (nickname, color, typing, last activity) only under `client.mu`.
  - Lock order: `room.Mutex` → `client.mu`, never the reverse.
  - NEVER close `client.Send`; lifecycle uses the idempotent `done` channel (`client.close()`).
  - Broadcasts use `client.trySend()` (non-blocking, shutdown-aware).
- New or changed server behavior MUST come with race-tested integration tests in `server/websocket_test.go` (real WebSocket clients against `handleWebSocket`) and `server/bootstrap_test.go` (HTTP bootstrap routes).
- Rune-safe truncation (`truncateRunes`) — never byte-slice strings for nick/text truncation.
- Server-side input sanitization strips ANSI escapes and control characters.
- Only `message` type is broadcast from clients; all other client frames are ignored.
- Room codes are 4 chars (case-insensitive, A-Z0-9) — changing `RoomCodeLength` in `shared/constants.go` touches every consumer.

## Protocol / flow

- Client connects to `/ws`, first frame MUST be `join` `{nick, room, password}`.
- First joiner becomes host; on host disconnect the next-oldest client succeeds.
- Room is deleted when empty; history capped at 30 messages.
- Rate limit: 5 frames/sec per client; idle clients disconnected after 30 min.
- `termchat discover --online` hits `/discover`; LAN discovery uses a UDP multicast beacon on `224.0.0.167:9999`.
- The server renders `server/scripts/bootstrap.sh` / `.ps1` with `{Room, BaseURL, Version}`; scripts download release binaries from GitHub and exec them with `--server` (derived from `PUBLIC_BASE_URL`). The CLI version is cached from the GitHub API every 5 min.
- Room codes are generated client-side (`shared.GenerateRoomCode()`); the CLI no longer calls any HTTP endpoint to create rooms.

## CI / release

- `.github/workflows/ci.yml` — PR gate: tidy, gofmt, vet, build, `go test -race`, 8-platform cross-compile.
- Tag `cli-v*` → `.github/workflows/cli.yml`: builds 8 binaries, generates `termchat-checksums.txt`, creates the GitHub Release via `gh`, then calls `aur.yml` (AUR package sync).
- `websocket.yml` — GHCR image on `main` push, path-filtered; manually dispatchable.
- Dependabot keeps `gomod` and `github-actions` dependencies updated; dependency review runs on every PR.
- Secrets: `AUR_SSH_PRIVATE_KEY`. Repo variables: `TERMCHAT_WS_URL` (baked into the CLI via ldflags `-X main.DefaultWS`).
- `main` is protected: PR + required checks (`lint-and-test`, `verify`). Never push directly.
- Commit signing is mandatory (`git commit -s -S`).

## Deployment

- `docker-compose.yml`: websocket + caddy + watchtower; config via `.env` (see `.env.example`).
- Caddy reverse proxies everything to websocket:8080; automatic HTTPS on termchat.sacred99.online.
- Server env: `WS_HOST`, `WS_PORT`, `PUBLIC_BASE_URL` (bootstrap scripts + binary redirects), `GITHUB_REPO`.

## Gotchas

- Bootstrap scripts live in `server/scripts/` (embedded into the server binary) — not at the repo root.
- The CLI's default server URL is compile-time baked; the bootstrap flow overrides it with `--server` derived from `PUBLIC_BASE_URL`.
- CHANGELOG entries for releases must be titled `## [cli-vX.Y.Z]` (the release workflow extracts notes by tag).
- `dist/` is gitignored; never commit binaries.
