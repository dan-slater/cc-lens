# The SSE stream

`GET /stream` returns a [Server-Sent Events][sse-spec] feed of every event
published to the bus. Each line in the response looks like:

```
id: 42
event: PostToolUse
data: {"id":42,"received_at":"2026-05-17T10:00:00Z","session_id":"abc","kind":"PostToolUse", ...}

```

Plus a `: ping\n\n` comment every 20 seconds for keepalive.

[sse-spec]: https://html.spec.whatwg.org/multipage/server-sent-events.html

## curl

```sh
curl -N -H "Authorization: Bearer $CC_LENS_TOKEN" http://127.0.0.1:8787/stream
```

`-N` (`--no-buffer`) is required — otherwise curl waits for a full line of
output before flushing.

## Browser

```js
// EventSource doesn't support custom headers, so disable auth or front
// cc-lens with a reverse proxy that injects the Authorization header.
const es = new EventSource("http://127.0.0.1:8787/stream");
es.addEventListener("PostToolUse", e => {
  const event = JSON.parse(e.data);
  console.log(event.session_id, event.kind);
});
es.addEventListener("error", e => console.warn("dropped", e));
```

For browser use through auth, terminate auth at a reverse proxy
([security.md](./security.md)).

## Node

```js
import { EventSource } from "undici"; // Node 22 has it as global too

const es = new EventSource("http://127.0.0.1:8787/stream", {
  dispatcher: undefined, // pass a custom dispatcher to add auth headers
});
es.onmessage = e => console.log(JSON.parse(e.data));
```

## Python

```python
import json, requests

with requests.get(
    "http://127.0.0.1:8787/stream",
    headers={"Authorization": f"Bearer {token}"},
    stream=True,
) as r:
    for line in r.iter_lines():
        if line.startswith(b"data: "):
            print(json.loads(line[6:]))
```

## Backpressure and gaps

The bus drops events for any subscriber whose 64-event buffer is full. The
client will observe **monotonically-increasing `id` values with gaps** —
that's the signal to reconcile via `GET /events?since_id=<last>` from the
ring buffer. There is no SSE retry/replay built in.

If you need lossless delivery, use the webhook fan-out and point it at a real
queue ([configuration.md](./configuration.md)).

## Reconnecting

Standard SSE clients (browser `EventSource`, `undici`, most libraries)
reconnect automatically and send `Last-Event-ID` on the new request. cc-lens
**does not** currently use `Last-Event-ID` for replay — the field is
accepted but ignored. To reconcile on reconnect, hit
`/events?since_id=<last seen id>` once and then resume the stream.

## Filtering

`/stream` ships every event. Filter client-side:

```sh
curl -N ... /stream \
  | grep -E '^event: (Stop|SubagentStop)$' -A2
```

If you only want one session, hit `/events?session_id=...` periodically
instead — there is no per-session SSE topic in v1.
