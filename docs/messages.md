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

```sh
cc-lens tmux-relay --server http://127.0.0.1:8787 --token $CC_LENS_TOKEN
```

This is a built-in consumer that treats tmux as the delivery channel:
1. Every 2s it asks cc-lens for the list of known sessions.
2. For each session id, it drains pending messages.
3. For each message, it runs:

   ```sh
   tmux send-keys -t claude:<session_id> "<body>" Enter
   ```

4. On success, it acks the message.

You must launch your Claude Code sessions in a tmux pane named
`claude:<session_id>` for this to work. The relay does not create panes
for you — that's a higher-level concern (and probably belongs in your shell
config or a launcher script).

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
