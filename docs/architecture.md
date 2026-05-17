# Architecture

```
┌──────────────┐   POST /events   ┌─────────────────────────────────────┐
│ Claude Code  │ ───────────────▶ │  cc-lens HTTP server (Go stdlib)    │
│ + hooks      │                  │                                     │
└──────────────┘                  │  in-memory event ring + SSE bus     │
                                  │  reads transcripts on demand        │
┌──────────────┐   GET /sessions  │  in-memory message queue            │
│ Any consumer │ ◀────GET /stream │  optional webhook fan-out           │
└──────────────┘ ◀──── …          └─────────────────────────────────────┘
                                              │
                                              ▼
                                   ~/.claude/projects/*/*.jsonl
                                   (Claude Code's own state, untouched)
```

## Components

| File                             | Responsibility                                                                  |
| -------------------------------- | ------------------------------------------------------------------------------- |
| `internal/server/server.go`      | `net/http` router + handlers.                                                   |
| `internal/server/events.go`      | Ring buffer + SSE fan-out (`Bus`).                                              |
| `internal/server/sessions.go`    | Disk lens over `~/.claude/projects` (`DiskSession`, `ReadTranscript`).          |
| `internal/server/messages.go`    | Per-agent FIFO message queue + ack (`Queue`).                                   |
| `internal/server/webhook.go`     | Outbound fan-out with one retry.                                                |
| `internal/server/auth.go`        | Bearer-token check with constant-time compare.                                  |
| `cmd/start.go`                   | `cc-lens start` — flag parsing + ListenAndServe.                                |
| `cmd/install_hooks.go`           | `cc-lens install-hooks` — patches `settings.json`.                              |
| `cmd/tmux_relay.go`              | `cc-lens tmux-relay` — reference queue consumer.                                |
| `main.go`                        | Subcommand dispatch.                                                            |

## Why a ring buffer, not a database?

cc-lens is a **lens**, not a store. Long-term history already exists in
Claude Code's own JSONL files; live fan-out only needs the recent past.
Adding persistent storage would:

- Double the source of truth (cc-lens copy vs. Claude Code's copy).
- Require migrations, retention policies, backups, schema decisions.
- Push us off the "zero-dep" path.

If you want a database, route through the webhook to one you control.

## Why SSE, not WebSocket?

- One-way is exactly what we need.
- Plain HTTP — survives proxies, no upgrade handshake.
- Standard reconnection semantics in every browser.
- Zero library code on either side.

The cost: no per-session topics — all subscribers receive everything. For
v1 the filter cost on the client is negligible. If you need server-side
fan-out by session, a webhook to your own router is the supported route.

## Concurrency

- `Bus` uses a `sync.RWMutex` around the ring slice; writes take the write
  lock briefly, reads take the read lock.
- Each SSE subscriber gets a 64-element buffered channel. Publish does a
  non-blocking send (`select { case ch <- e: default: }`) — slow subscribers
  drop events and observe a gap in `id`, never block the publisher.
- `Queue` uses a single `sync.Mutex`; throughput is fine because the queue is
  not on the hot path for typical agent work.
- `Webhook.Fire` spawns one goroutine per event. No bounded worker pool —
  consider adding one if you have a slow webhook and a high event rate.

## Transcript schema assumptions

We rely only on these JSONL fields:

- `type` (string, e.g. `user`, `assistant`, `tool_use`, `tool_result`)
- `uuid`, `parentUuid` (for ordering and pagination)
- `timestamp`
- `sessionId`

Everything else is preserved opaquely in the `raw` JSON of each
`TranscriptLine`. This is the smallest contract we can plausibly hold
across Claude Code versions; see the [Phase 0 research](#phase-0-research)
for evidence that these fields are stable across the tools that depend on
them today.

## Phase 0 research

Before any code, a survey of the prior art:

- `NirDiamant/claude-watch` (took the name; SQLite + UI dashboard, not a lens).
- `disler/claude-code-hooks-multi-agent-observability` (1.4k★, Python+Bun+
  SQLite+Vue, no license — proves demand, wrong shape).
- `simple10/agents-observe` (558★, MIT — Docker+SQLite+React).
- `darshannere/observagent` (closest neighbour; bundled UI).
- `ryoppippi/ccusage` (14k★ JSONL cost analyzer — confirms `usage` field is
  the right way to do token accounting).
- Anthropic's own surface: `@anthropic-ai/claude-code`,
  `@anthropic-ai/claude-agent-sdk`. No hook ingest server, no SSE bus, no
  session-listing API. The gap cc-lens fills is real.

Conclusion: build, but as a thin transport layer that other dashboards can
sit on top of, rather than another bundled UI.
