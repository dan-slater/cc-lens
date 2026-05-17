# Security model

cc-lens is deliberately small, which means the security model is small too.
Read this before exposing it beyond `127.0.0.1`.

## Threat model

cc-lens is designed to be reachable by **trusted hook senders on machines
you control**. It is not designed to be exposed on the public internet,
multi-tenant, or to defend against authenticated abuse.

## Authentication

A single shared bearer token via `--token` / `CC_LENS_TOKEN`.

- Comparison uses `crypto/subtle.ConstantTimeCompare`, so the check is not
  timing-leaky.
- Empty token disables auth. **Never do this on a non-loopback bind.**
- No per-agent tokens, no rotation, no allowlist. Rotating the token
  requires restarting cc-lens and re-running `install-hooks`.

## Transport

cc-lens speaks plain HTTP. There is no built-in TLS. For anything more than
a loopback bind:

```
agent → cc-lens (127.0.0.1:8787)  ←  Caddy / nginx / Cloudflare Tunnel → public
```

Example Caddyfile:

```caddy
cc-lens.example.com {
  reverse_proxy 127.0.0.1:8787
}
```

If your consumers can't supply an `Authorization` header (looking at you,
browser `EventSource`), terminate auth at the proxy:

```caddy
cc-lens.example.com {
  @authed header Cookie *cc_lens_session=*
  handle @authed {
    reverse_proxy 127.0.0.1:8787 {
      header_up Authorization "Bearer {env.CC_LENS_TOKEN}"
    }
  }
  respond 401
}
```

## What cc-lens reads from disk

- `~/.claude/projects/**/*.jsonl` — read on demand to populate `/sessions`
  and `/sessions/{id}/transcript`. Never modified.
- `~/.claude/settings.json` — read and rewritten by `install-hooks` only.
  Other subcommands do not touch it.
- The webhook URL is read at startup; cc-lens makes outbound HTTPS calls to
  it if configured.

## Hard request limits

- `POST /events` body is capped at **1 MiB**. Oversize requests get a 400.
- SSE subscribers have a 64-event in-process buffer; slow consumers see
  gaps, not OOM.
- `ReadHeaderTimeout` is 5s on the listener.

## What cc-lens does **not** do

- No rate limiting (rely on a reverse proxy).
- No request logging beyond Go's default and webhook delivery errors.
- No audit log of who acked which message.
- No encryption of in-memory data.

## Reporting issues

Email the repo owner. Please don't file public issues for vulnerabilities
that materially help an attacker before a fix is shipped.
