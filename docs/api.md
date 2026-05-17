# HTTP API reference

Base URL: whatever you started `cc-lens` on (default `http://127.0.0.1:8787`).
All endpoints except `/healthz` require `Authorization: Bearer $TOKEN` when
a token is configured.

## Status codes

| Code | Meaning                                            |
| ---- | -------------------------------------------------- |
| 200  | OK                                                 |
| 202  | Accepted (event/message enqueued)                  |
| 204  | No content (ack succeeded)                         |
| 400  | Bad request (missing `kind`, invalid JSON, etc.)   |
| 401  | Unauthorized                                       |
| 404  | Not found                                          |
| 500  | Server error                                       |

## `GET /healthz`

No auth. Returns `"ok"` with status 200. Use for liveness probes.

## `POST /events`

Ingest a hook event. Body must be JSON. Required field: `kind`. Recognized
top-level fields (anything else is preserved opaquely):

| Field        | Type   | Purpose                                                  |
| ------------ | ------ | -------------------------------------------------------- |
| `kind`       | string | Hook kind (`PreToolUse`, `Stop`, etc.). **Required.**    |
| `session_id` | string | Claude Code session id.                                  |
| `cwd`        | string | Working directory the agent was launched in.             |
| `workspace`  | string | Free-form label set by `install-hooks --workspace`.      |
| `host`       | string | Source hostname (set by the installed hook).             |
| `label`      | string | Free-form label.                                         |

Response: `202 {"id": <ring-buffer-id>}`.

The full request body is retained in the in-memory ring as `raw`.

## `GET /events`

Return events from the in-memory ring buffer.

| Query        | Type   | Default | Notes                                                  |
| ------------ | ------ | ------- | ------------------------------------------------------ |
| `session_id` | string | *(all)* | Filter to one session.                                 |
| `since_id`   | uint64 | `0`     | Return only events with `id > since_id`.               |
| `limit`      | int    | `0`     | If > 0, return only the most recent N matching events. |

Response: `[Event...]` (oldest → newest within the returned window).

## `GET /sessions`

Merged view of every known session: union of (a) sessions seen via the
hook ingest in this process's lifetime, (b) `.jsonl` files on disk under
`~/.claude/projects` (or `$CC_LENS_PROJECTS_DIR`).

Response: array of `SessionRow`:

```json
{
  "id": "abc-123",
  "last_kind": "PostToolUse",
  "last_ts": "2026-05-17T10:00:00Z",
  "cwd": "/Users/me/proj",
  "workspace": "my-workspace",
  "host": "macbook",
  "label": "",
  "transcript_path": "/Users/me/.claude/projects/-Users-me-proj/abc-123.jsonl",
  "transcript_bytes": 184320,
  "modified_at": "2026-05-17T09:59:42Z"
}
```

A session that has only ever sent hooks (no disk transcript yet) is missing
the `transcript_*` and `modified_at` fields. A session known only from disk
is missing `last_kind` / `last_ts`.

## `GET /sessions/{id}`

Same shape as one row of `/sessions`. `404` if neither source knows the id.

## `GET /sessions/{id}/transcript`

Read and parse the on-disk `.jsonl` for one session.

| Query    | Type   | Notes                                                  |
| -------- | ------ | ------------------------------------------------------ |
| `limit`  | int    | If > 0, return only the most recent N lines.           |
| `before` | string | Reverse-paginate: return lines before this `uuid`.     |

Response: array of `TranscriptLine`:

```json
{
  "type": "assistant",
  "uuid": "u2",
  "parentUuid": "u1",
  "timestamp": "2026-05-17T10:00:00Z",
  "sessionId": "abc",
  "raw": { ... original JSON ... }
}
```

Only `type`, `uuid`, `parentUuid`, `timestamp`, `sessionId` are projected.
Everything else stays inside `raw` so you can introspect message content,
tool calls, usage, etc. without cc-lens needing to know the full schema.

## `GET /stream` — Server-Sent Events

See [sse.md](./sse.md).

## `POST /agents/{id}/messages`

Queue a message addressed to an agent. Body: `{"body": "..."}`.

Response: `202` with the queued message:

```json
{
  "id": "8f2c3a...",
  "agent_id": "abc",
  "body": "continue with the next task",
  "created_at": "2026-05-17T10:00:00Z",
  "delivered": false
}
```

Delivery is the consumer's responsibility. See [messages.md](./messages.md).

## `GET /agents/{id}/messages`

Return pending (not-yet-acked) messages for an agent, FIFO. No side effects.

## `POST /messages/{id}/ack`

Mark a message delivered and remove it from the per-agent FIFO. Response:
`204` on success, `404` if the id is unknown.
