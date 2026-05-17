# Sending messages to agents

cc-lens is delivery-agnostic. It accepts messages onto a per-agent FIFO and
hands them out when asked. **How a message actually gets typed into a running
Claude Code session is up to you.**

## Lifecycle

```
producer        cc-lens         consumer        agent
   │               │               │              │
   │ POST /agents/abc/messages    │              │
   │  {"body": "do X"}            │              │
   │──────────────▶│              │              │
   │   202 {id}    │              │              │
   │◀──────────────│              │              │
   │               │ GET /agents/abc/messages    │
   │               │◀─────────────│              │
   │               │  [{id, body, ...}]          │
   │               │─────────────▶│              │
   │               │              │ (deliver)    │
   │               │              │─────────────▶│
   │               │ POST /messages/{id}/ack     │
   │               │◀─────────────│              │
   │               │     204      │              │
```

Acking is mandatory — until a message is acked it stays at the head of the
FIFO and will be re-returned by the next `GET`.

## In-process consumer (any language)

Anything that can poll an HTTP endpoint can be a consumer. Pseudocode:

```python
while True:
    msgs = http.get(f"{server}/agents/{agent_id}/messages").json()
    for m in msgs:
        deliver(m["body"])               # your code
        http.post(f"{server}/messages/{m['id']}/ack")
    time.sleep(2)
```

## The `tmux-relay` subcommand

`cc-lens tmux-relay` is a built-in consumer that types queued messages
into the right tmux window — so anything you `POST /agents/<id>/messages`
appears in Claude Code as if you typed it.

There's a chicken-and-egg problem to solve first: **Claude Code generates
the session id at startup**, so you can't pre-name a tmux window for it.
We solve that with a one-line `SessionStart` hook that renames the current
tmux window the moment Claude starts.

### One-time setup (≈ 30 seconds)

**1. Add the rename hook.** Open `~/.claude/settings.json` (or run
`cc-lens install-hooks` first to scaffold it), and add a `SessionStart`
entry that renames whichever tmux window Claude is launched in:

```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "id=$(jq -r '.session_id'); [ -n \"$TMUX\" ] && tmux rename-window \"claude:$id\" 2>/dev/null; echo '{\"continue\":true}'"
          }
        ]
      }
    ]
  }
}
```

This runs on every `SessionStart`. If you're not inside tmux (`$TMUX`
unset), it's a no-op. The trailing `echo` keeps Claude Code happy — hook
output is JSON-parsed.

> **Already running `cc-lens install-hooks`?** That installer overwrites
> the `hooks` block. Either merge this entry manually after running it, or
> add a `--with-tmux-rename` flag request as a feature ask.

**2. Start the relay** in any terminal (this can run on the same machine
as cc-lens or anywhere it can reach the HTTP API):

```sh
cc-lens tmux-relay \
  --server http://127.0.0.1:8787 \
  --token "$CC_LENS_TOKEN"
```

The relay polls every 2 seconds (`--interval 2s` to change). Leave it
running — it's a long-lived process.

**3. Launch Claude inside a tmux window.** Anywhere:

```sh
tmux new-session -A -s work     # attach if it exists, else create
claude
```

The moment Claude prints its first line, the `SessionStart` hook fires
and your tmux window gets renamed to `claude:<id>`. Verify:

```sh
tmux list-windows -F '#{window_name}'
# → claude:0b7e3a4f-...
```

### Sending a message

Find the session id (from `cc-lens-ui`, or `curl /sessions`), then:

```sh
curl -X POST "http://127.0.0.1:8787/agents/0b7e3a4f-.../messages" \
  -H "Authorization: Bearer $CC_LENS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"body":"please continue with the next task"}'
```

Within ≤ 2 seconds the relay polls, sees the message, runs
`tmux send-keys -t claude:0b7e3a4f-... "please continue..." Enter`, and
acks it. From Claude's perspective it's exactly as if you typed it.

### Verifying end-to-end

If a message doesn't arrive, walk this checklist:

1. **Is the relay running?** `ps aux | grep tmux-relay`. If it died,
   check its stderr — usually a 401 (wrong token) or a 404 (wrong server URL).
2. **Is the window named correctly?**
   ```sh
   tmux list-windows -F '#{window_name}'
   ```
   You should see exactly `claude:<session-id>`. If you see just
   `claude` or an integer, the rename hook didn't fire — make sure you
   started Claude *inside* a tmux session (`echo $TMUX` should print a
   path).
3. **Is the message in the queue?**
   ```sh
   curl -H "Authorization: Bearer $CC_LENS_TOKEN" \
     "http://127.0.0.1:8787/agents/<id>/messages"
   ```
   Should show your message until the relay acks it.
4. **Try the raw `tmux send-keys`** to bypass the relay:
   ```sh
   tmux send-keys -t "claude:<id>" "test from CLI" Enter
   ```
   If that doesn't appear in Claude either, your tmux target is wrong;
   if it does, the relay isn't seeing it (check its logs).

### Launcher shortcut

To avoid the two-step `tmux new-session && claude` dance, drop this in
your shell config:

```bash
# zsh / bash — open a fresh tmux window and launch claude inside it
cc() {
  if [ -z "$TMUX" ]; then
    tmux new-session -A -s claude "claude $*"
  else
    tmux new-window "claude $*"
  fi
}
```

Now `cc` from anywhere always starts Claude in a tmux window the relay
can find.

### What the relay actually does

```sh
# every --interval (default 2s):
for session_id in $(cc-lens GET /sessions | jq -r '.[].id'); do
  for msg in $(cc-lens GET /agents/$session_id/messages); do
    tmux send-keys -t "claude:$session_id" "$body" Enter \
      && cc-lens POST /messages/$id/ack
  done
done
```

That's the entire reference implementation in ~100 lines of Go. See
[`cmd/tmux_relay.go`](../cmd/tmux_relay.go) if you want to port it to a
language that fits your stack better, or replace tmux with screen / a
WezTerm pane / an IDE PTY / a Slack bot — the queue API is delivery-agnostic.

## Why no built-in delivery?

Delivery is the part of agent orchestration that varies the most across
setups: tmux, GNU screen, an IDE-attached PTY, a voice client, a Slack bot,
a CI runner. Baking any one of these in would force the wrong choice on
everyone else. Keeping the queue HTTP-only and writing relays as separate
consumers means a five-line Python script can plug in any new delivery
mechanism.

## Implementing a new relay

A relay is just a polling consumer that:
1. Has some way to map an agent_id to a delivery target (tmux pane, IDE
   session, etc.).
2. Translates `body` into whatever input that target expects.
3. Acks on successful delivery, retries (or doesn't) on failure.

See [`cmd/tmux_relay.go`](../cmd/tmux_relay.go) for the reference impl —
it's under 100 lines.

## Durability caveat

The queue is in-memory. Restarting `cc-lens` drops every un-acked message.
If you need persistence, either:
- Persist on the producer side and don't enqueue until you're sure cc-lens
  is up.
- Mirror queued messages to a real queue (Redis, SQS, NATS) and treat
  cc-lens's queue as a cache.
