# Configuration

Every flag has an environment-variable equivalent. Flags win when both are
set.

| Flag              | Env                     | Default              | Notes                                                             |
| ----------------- | ----------------------- | -------------------- | ----------------------------------------------------------------- |
| `--addr`          | `CC_LENS_ADDR`          | `:8787`              | Listen address. Bind `127.0.0.1:8787` for local-only.             |
| `--token`         | `CC_LENS_TOKEN`         | *(empty)*            | Bearer token. Empty = no auth. See [security.md](./security.md).  |
| `--ring`          | `CC_LENS_RING_SIZE`     | `1000`               | Event ring buffer capacity. See "Sizing the ring" below.          |
| `--webhook`       | `CC_LENS_WEBHOOK_URL`   | *(disabled)*         | If set, every matched event is POSTed here.                       |
| `--webhook-kinds` | `CC_LENS_WEBHOOK_KINDS` | *(all)*              | Comma-separated list of kinds to fan out. Empty = all.            |
| *(test only)*     | `CC_LENS_PROJECTS_DIR`  | `~/.claude/projects` | Override the on-disk transcript root.                             |

## Sizing the ring

The ring buffer holds the most recent N events in memory. Default 1000.

- A single Claude Code session emits roughly **5–30 events per turn** depending
  on which hooks you have configured (PreToolUse + PostToolUse dominate).
- A heavily-used workstation with a handful of concurrent agents over a few
  hours can produce 10k+ events. If `/events` keeps returning a thin tail,
  bump `--ring 10000`.
- The buffer is `sizeof(Event)` × N. Events are small (a few hundred bytes
  plus the original JSON body, capped at 1 MiB per event). 10k events is on
  the order of a few MiB of RAM.

## Webhook semantics

When `--webhook` is set, every event whose `kind` matches the filter is
POSTed to the URL as JSON, in the same shape returned by `/events`. Delivery
is **fire-and-forget with one retry** and a 5-second timeout per attempt.
There is no persistent queue — if your webhook is down for a minute, those
events are lost from the webhook stream (but still in the in-memory ring and
on `/stream`).

For at-least-once delivery, point your webhook at a small relay you control
(e.g. an SQS putter, a Kafka producer) rather than your application directly.

## Disabling auth

Setting `--token ""` (the default) disables bearer-token auth. This is
appropriate for `127.0.0.1` binds on a developer workstation only. See
[security.md](./security.md) before exposing cc-lens beyond `localhost`.

## Multiple instances

Run more than one `cc-lens` on different ports and label each hook install
with a `--workspace` so events from different cc-lens deployments are
distinguishable. The hook installer writes the `workspace` value into every
event body.
