# cc-lens docs

Start here, then drill in.

- **[install.md](./install.md)** — go install, prebuilt binaries, systemd.
- **[configuration.md](./configuration.md)** — flags, env vars, ring sizing,
  webhook semantics.
- **[hooks.md](./hooks.md)** — what `install-hooks` does, manual install,
  troubleshooting.
- **[api.md](./api.md)** — full HTTP reference with request/response shapes.
- **[sse.md](./sse.md)** — consuming `/stream` from curl, the browser,
  Node, Python; backpressure and gap recovery.
- **[messages.md](./messages.md)** — the agent message queue, the
  `tmux-relay` reference consumer, writing your own relay.
- **[security.md](./security.md)** — threat model, auth, putting cc-lens
  behind a reverse proxy.
- **[architecture.md](./architecture.md)** — components, concurrency,
  design trade-offs.
