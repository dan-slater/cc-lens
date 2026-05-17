---
name: stuck-session-shepherd
description: Every 15 minutes, find Claude Code sessions that have been idle waiting on the user and queue a short nudge to each one.
schedule:
  cron: "*/15 * * * *"
  timezone: "Europe/London"
cc_lens:
  reads:  [GET /sessions]
  writes: [POST /agents/{id}/messages]
inputs: []
outputs:
  channel: log
  format: text
---

# Stuck-session shepherd

The flagship cc-lens-native example, and the reason the `POST
/agents/{id}/messages` endpoint exists at all. Every 15 minutes the
shepherd asks cc-lens "which sessions look idle in a way that's blocking
on me?", and for each one queues a short message that types itself back
into the session through `cc-lens tmux-relay`. From the Claude session's
point of view it's exactly as if the user wandered back and asked.

It's a closed-loop autopilot: the agent stalls waiting on a human; the
shepherd is a human-shaped poke; the agent answers and proceeds. Without
cc-lens you would need to remember to come back to a session that's
sitting on a `Notification` hook for twenty minutes. With the shepherd
you don't.

## Criteria for "stuck"

A row from `GET /sessions` is considered stuck if **all** of:

- `last_kind` is `Notification` or `UserPromptSubmit` (the two states
  that mean "we are waiting on the user"),
- `last_ts` is more than 10 minutes ago,
- `last_ts` is less than 12 hours ago (older than that is a dead session,
  not a stuck one — leave it alone),
- the session doesn't already have a pending message in its queue (a
  quick `GET /agents/{id}/messages` confirms — otherwise the relay hasn't
  caught up yet and nudging again just spams).

## Prompt

The shepherd doesn't need a model in the loop for the common case — the
nudge text is fixed. If you want a model to write a *customised* nudge
based on the session's recent transcript, see the optional step below.

Default nudge body:

```
Any update? Reply `continue`, `done`, or describe the blocker. (Nudge
from the cc-lens shepherd — last hook was {{last_kind}} {{relative_ts}}.)
```

Optional, model-written variant (one curl extra per stuck session):

```
You are writing a one-line nudge for a Claude Code session that has been
idle for {{relative_ts}}. The last hook was {{last_kind}}. The most
recent 20 transcript lines are:

{{transcript_tail}}

Write a single sentence asking the user to unblock the session.
Reference the specific thing it appears to be waiting on. End with the
literal text: "Reply `continue`, `done`, or describe the blocker."
```

## cc-lens calls

```sh
# 1. Inventory.
sessions=$(curl -sH "Authorization: Bearer $CC_LENS_TOKEN" \
  "$CC_LENS_URL/sessions")

# 2. Pick stuck rows. (jq shown for brevity — equivalent awk/sed works.)
now=$(date -u +%s)
echo "$sessions" | jq -r --argjson now "$now" '
  .[] | select(
    (.last_kind == "Notification" or .last_kind == "UserPromptSubmit")
    and ($now - (.last_ts | fromdateiso8601)) > 600
    and ($now - (.last_ts | fromdateiso8601)) < 43200
  ) | "\(.id)\t\(.last_kind)\t\(.last_ts)"
' | while IFS=$'\t' read -r id last_kind last_ts; do

  # 3. Skip if the queue isn't drained yet.
  pending=$(curl -sH "Authorization: Bearer $CC_LENS_TOKEN" \
    "$CC_LENS_URL/agents/$id/messages" | jq 'length')
  [ "$pending" -gt 0 ] && continue

  # 4. Queue the nudge.
  curl -sX POST "$CC_LENS_URL/agents/$id/messages" \
    -H "Authorization: Bearer $CC_LENS_TOKEN" \
    -H "Content-Type: application/json" \
    -d "{\"body\":\"Any update? Reply \\\`continue\\\`, \\\`done\\\`, or describe the blocker. (Nudge from the cc-lens shepherd — last hook was $last_kind.)\"}"

done
```

## Delivery

The shepherd only enqueues. Actual delivery is handled by `cc-lens
tmux-relay` (or any other consumer you wrote against
[messages.md](https://github.com/dan-slater/cc-lens/blob/main/docs/messages.md)).
If the relay isn't running, the message sits in the queue and gets
delivered the moment it comes back up — no shepherd retry needed.

## Adapting this

- **Tune the 10-minute / 12-hour window** to your habit. A 5-minute
  window pings too often; a 30-minute window misses sessions you forgot
  about over lunch.
- **Quiet hours.** Wrap the routine in a `[ "$(date +%H)" -ge 22 ] &&
  exit 0` guard if you don't want to nudge sessions while you sleep.
- **Disable for specific sessions.** Add `[shepherd:off]` to the session
  `label` field via your own tooling, and filter on it in step 2.
