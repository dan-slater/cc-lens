# cc-lens

> A zero-dependency HTTP/SSE lens over running Claude Code sessions.

`cc-lens` is the unopinionated transport layer that observability dashboards,
voice clients, IDE plugins, and orchestrators sit on top of. It does not store
your transcripts — Claude Code already does that, and `cc-lens` reads them on
demand. It does not ship a frontend. It does not have a database. It has no
production dependencies.

| Property        | Value                                                           |
| --------------- | --------------------------------------------------------------- |
| Language        | Go 1.24+                                                        |
| Production deps | **zero** (Go stdlib only)                                       |
| Binary size     | ~7 MB, static                                                   |
| Storage         | in-memory ring buffer (events) + in-memory queue (messages)     |
| Transcripts     | read on demand from `~/.claude/projects/*.jsonl` (never copied) |
| License         | **FSL-1.1-ALv2** (becomes Apache 2.0 after 2 years) — see below |

## Why does this exist?

Claude Code emits [hook events][hooks] (outbound HTTP POSTs) and writes
[per-session JSONL transcripts][dotclaude] to disk. There is no built-in
receiver, no SSE bus, and no programmatic way for an external process to ask
"which sessions are running and what just happened in them?"

Several projects fill parts of this gap — `disler/claude-code-hooks-multi-agent-observability`,
`simple10/agents-observe`, `observagent` — but they all bundle a UI, a database,
and a delivery mechanism. `cc-lens` is the layer *below* those products: the
thinnest possible HTTP surface that turns Claude Code into something you can
program against from any language, with one static binary.

[hooks]: https://docs.claude.com/en/docs/claude-code/hooks
[dotclaude]: https://docs.claude.com/en/docs/claude-code/settings

## Install

```sh
go install github.com/dan-slater/cc-lens@latest
```

Or grab a prebuilt binary from the [releases page][releases]. Full options
(systemd, manual builds, cross-compile): see **[docs/install.md](./docs/install.md)**.

[releases]: https://github.com/dan-slater/cc-lens/releases

## Quickstart

```sh
# 1. Start the server. Token + addr explained in docs/configuration.md.
CC_LENS_TOKEN=hunter2 cc-lens start --addr :8787

# 2. Patch ~/.claude/settings.json so Claude Code posts events back.
#    What this writes, manual install, troubleshooting: docs/hooks.md.
cc-lens install-hooks --server http://127.0.0.1:8787 --token hunter2

# 3. Open a Claude Code session in another terminal.
claude

# 4. Watch events stream in. Browser/Node/Python clients + backpressure: docs/sse.md.
curl -N -H "Authorization: Bearer hunter2" http://127.0.0.1:8787/stream
```

Each step has a deeper page:
1. **Start** → [configuration.md](./docs/configuration.md) — flags, env vars, ring sizing, webhook fan-out.
2. **Hooks** → [hooks.md](./docs/hooks.md) — what gets written, per-project install, manual install, debugging.
3. **Claude Code session** → no cc-lens-side config; if events don't arrive, see the [troubleshooting section of hooks.md](./docs/hooks.md#troubleshooting).
4. **Stream** → [sse.md](./docs/sse.md) — `EventSource`, `curl -N`, reconnection, the gap caveat.

Want to send messages *into* an agent? → [messages.md](./docs/messages.md).
Exposing cc-lens beyond localhost? → [security.md](./docs/security.md).

## HTTP API

All endpoints require `Authorization: Bearer $CC_LENS_TOKEN` when a token is
configured. If the token is empty, auth is disabled (useful locally; never do
this on a droplet).

| Method | Path                                                  | Purpose                                                          |
| ------ | ----------------------------------------------------- | ---------------------------------------------------------------- |
| GET    | `/healthz`                                            | Liveness. No auth required.                                      |
| POST   | `/events`                                             | Hook ingest. Validates `kind`, pushes to ring + SSE.             |
| GET    | `/events?session_id=&since_id=&limit=`                | Recent events from the ring.                                     |
| GET    | `/sessions`                                           | Merged view: live hooks ∪ on-disk transcripts.                   |
| GET    | `/sessions/{id}`                                      | One session.                                                     |
| GET    | `/sessions/{id}/transcript?limit=&before=`            | Tail of the JSONL transcript.                                    |
| GET    | `/stream`                                             | Server-Sent Events of every published event. 20s keepalive ping. |
| POST   | `/agents/{id}/messages`                               | Queue a message for an agent. Body: `{"body": "..."}`.           |
| GET    | `/agents/{id}/messages`                               | Drain pending messages for an agent.                             |
| POST   | `/messages/{id}/ack`                                  | Mark a message delivered.                                        |

### Example: ingest

```sh
curl -X POST http://127.0.0.1:8787/events \
  -H "Authorization: Bearer hunter2" \
  -H "Content-Type: application/json" \
  -d '{"kind":"Stop","session_id":"abc","cwd":"/Users/me/proj"}'
# → 202 {"id":1}
```

### Example: SSE

```
event: Stop
id: 1
data: {"id":1,"received_at":"2026-05-17T10:00:00Z","session_id":"abc","kind":"Stop",...}
```

### Example: send a message to an agent

```sh
curl -X POST http://127.0.0.1:8787/agents/abc/messages \
  -H "Authorization: Bearer hunter2" \
  -H "Content-Type: application/json" \
  -d '{"body":"continue with the next task"}'
```

How the message gets delivered to the agent is up to you. The queue is
delivery-agnostic. `cc-lens tmux-relay` is one such consumer (it `send-keys`es
the body into a tmux pane named `claude:<session_id>`). You can write your own
in any language.

## Configuration

All flags accept env-var equivalents:

| Flag                       | Env                       | Default                 |
| -------------------------- | ------------------------- | ----------------------- |
| `--addr`                   | `CC_LENS_ADDR`            | `:8787`                 |
| `--token`                  | `CC_LENS_TOKEN`           | *(empty = no auth)*     |
| `--ring`                   | `CC_LENS_RING_SIZE`       | `1000`                  |
| `--webhook`                | `CC_LENS_WEBHOOK_URL`     | *(disabled)*            |
| `--webhook-kinds`          | `CC_LENS_WEBHOOK_KINDS`   | *(all kinds)*           |
| *(test override)*          | `CC_LENS_PROJECTS_DIR`    | `~/.claude/projects`    |

## Architecture (one mental model)

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

## Design notes

- **Why a ring buffer, not a DB?** The point of `cc-lens` is to be a *lens*, not a
  store. Long-term history lives in Claude Code's own JSONL files; live
  fan-out lives in memory. If you want a database, point your webhook at one.
- **Why SSE, not WebSocket?** SSE is one-way, plain HTTP, survives proxies,
  reconnects for free in browsers, and needs no library. We don't need a duplex
  channel.
- **Why Go?** Single static binary, stdlib HTTP+SSE is honest and complete,
  goroutines+channels make the SSE bus and per-agent queues trivial. No
  bundlers, no lockfiles, no transitive deps to audit.
- **Transcript schema is "permissive."** We only key on `type`, `sessionId`,
  `uuid`, `parentUuid`, `timestamp` — fields that have been stable across all
  observed Claude Code releases. Everything else is preserved opaquely in
  `raw` so consumers can introspect.

## Limitations / non-goals (v1)

- **No persistence** — restart drops the event ring and the message queue.
- **No multi-tenancy** — one shared bearer token.
- **No token counting** — `message.usage` is already in the JSONL Claude Code
  writes; surface it from `/sessions/{id}/transcript` if you need it.
- **No frontend.** Examples in this README are the demo.
- **No Windows hook installer testing yet** — the path (`~/.claude/settings.json`
  resolves via `os.UserHomeDir`) is correct, but the generated `jq | curl`
  hook command assumes a Unix shell.

## Example scheduled jobs

Worked examples of recurring jobs that talk to cc-lens — daily briefs, a
"stuck-session shepherd" that nudges idle agents, a session digest — live
on the long-lived [`examples`](https://github.com/dan-slater/cc-lens/tree/examples/examples/schedules)
branch (not `main`). Each is a markdown file with YAML frontmatter
(schedule, inputs, cc-lens calls used) plus the exact Claude prompt and
curl commands. A small `run.sh` helper extracts the runnable bits.

```sh
git fetch origin examples
git checkout examples
ls examples/schedules/
```

They run anywhere that can reach your cc-lens server: Anthropic
`/schedule` routines, systemd timers, GitHub Actions cron. The branch
deliberately stays out of `main` so feature additions and tagged releases
don't drag operational examples along.

## Development

```sh
go build ./...
go test ./...
go vet ./...
```

All tests are stdlib (`net/http/httptest`, `testing`); no test deps.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md). TL;DR: keep the dep tree empty,
stay under ~500 LoC of production code, prefer one clear stdlib call over a
dependency.

## License

**[Functional Source License, v1.1, ALv2 future grant](./LICENSE)** (FSL-1.1-ALv2).

In plain English:
- **You can use, modify, and redistribute cc-lens** for any purpose other than
  *competing* with it — i.e. you cannot offer cc-lens (or a substitute for it)
  as a hosted commercial service or repackaged commercial product.
- **Internal use, education, research, and professional services** built on
  cc-lens are explicitly permitted.
- **After two years**, each release automatically converts to Apache 2.0 —
  fully OSI-open, no strings.

This is a [Fair Source](https://fair.io/) license, not OSI-approved
"open source". The trade-off is intentional: we want cc-lens to be reliably
single-vendor today so the project stays small and coherent, and reliably
permissive tomorrow so it never becomes a hostage to that vendor.

See [SECURITY.md](./SECURITY.md) for our supply-chain posture (signed
releases, build provenance, branch protection, zero deps).
