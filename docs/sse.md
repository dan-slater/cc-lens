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

`EventSource` cannot set custom headers, so cc-lens also accepts the bearer
token as a `?token=` query parameter (on every endpoint, but you only need
it here):

```js
const url = `http://127.0.0.1:8787/stream?token=${encodeURIComponent(token)}`;
const es = new EventSource(url);
es.addEventListener("PostToolUse", e => {
  const event = JSON.parse(e.data);
  console.log(event.session_id, event.kind);
});
es.addEventListener("error", e => console.warn("dropped", e));
```

Trade-off: query-param tokens can land in proxy logs and browser history. If
you need to avoid that, terminate auth at a reverse proxy and have it
inject `Authorization` from a cookie — see [security.md](./security.md).

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

## Cloudflare Tunnel + HTTP/3 gotcha

**Symptom.** You front cc-lens with `cloudflared` (or any Cloudflare-proxied
hostname). Browsers connect fine, the first events arrive, then the stream
goes silent. `EventSource` flips to `readyState=0`, reconnects, repeats. No
errors in the cc-lens log; nothing obviously wrong in the tunnel log.

**Cause.** Cloudflare's edge closes HTTP/3 (and HTTP/2) streams that look
idle to it. SSE comment lines (`: ping`) count as activity, but the **inner**
idle timeout on whatever process actually terminates the connection has to
be longer than the keepalive interval, or the server hangs up before the
next ping.

**Recipe.**

1. Keep cc-lens's 20s keepalive on (it already pings every 20s — see the
   `: ping\n\n` comment above). Don't disable it behind a proxy.
2. If you're terminating the tunnel at a non-Go process before cc-lens
   (e.g. a Bun reverse proxy in front), set its idle timeout to **>90s** —
   for Bun, `Bun.serve({ idleTimeout: 255 })` is what works in practice.
   Go's `net/http` has no idle timeout by default, so the stock cc-lens
   binary needs no change.
3. In `cloudflared` config, leave `proxyConnectTimeout` /
   `proxyTLSTimeout` at defaults but bump `--http2-origin` off if you see
   ALPN-related disconnects; HTTP/1.1 to the origin is fine for SSE.

If you can still reproduce drops after that, run the curl recipe above
against the public hostname — if curl stays connected and the browser
doesn't, the issue is on the browser/CF side (often QUIC); add
`?cf_quic=off` to your test or disable HTTP/3 for the hostname while
debugging.

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
