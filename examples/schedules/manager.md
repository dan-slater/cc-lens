---
name: manager
description: Every 15 minutes, review every running Claude session against the grand plan and queue a tailored nudge (check-in, redirect, or stop) where one is warranted.
schedule:
  cron: "*/15 * * * *"
  timezone: "Europe/London"
cc_lens:
  reads:  [GET /sessions, GET /sessions/{id}/transcript]
  writes: [POST /agents/{id}/messages]
inputs:
  - kind: file
    description: "the grand plan — a plain-text markdown file describing the overall objective, named sub-goals, and which agents (by cwd or label) own what. Path is configurable; default `~/.cc-lens/plan.md`."
outputs:
  channel: log
  format: text
---

# Manager

The flagship cc-lens-native example, and the reason
`POST /agents/{id}/messages` exists at all. Every 15 minutes the manager
re-reads **the grand plan**, walks **every** running Claude session, and
asks one question: *given the plan and what this agent has done in the
last few turns, should I say anything?*

It is not a heartbeat or a stuck-session reaper. It is an **LLM-driven
supervisor** that uses cc-lens as its eyes (`/sessions` +
`/sessions/{id}/transcript`) and hands (`/agents/{id}/messages`). When it
decides to speak, the message types itself into the running session via
[`cc-lens tmux-relay`](https://github.com/dan-slater/cc-lens/blob/main/docs/messages.md)
— from the agent's point of view it's exactly as if the user wandered
back and gave new direction.

Four decisions per agent, per tick:

| Decision | When | What it sends |
| --- | --- | --- |
| `ok`       | agent is on plan and making progress | nothing |
| `checkin`  | agent has been idle past the threshold and not on a `Stop` | a short status request |
| `redirect` | agent is working on the wrong thing relative to the plan | a redirect with the right next step |
| `stop`     | agent is actively doing harm or has fully wandered off | a polite stop and ask-for-confirmation |

The `ok` case is the common one — the manager should mostly say nothing.
A chatty manager that pings every tick will be silenced by the user
within an hour. Bias toward silence.

## The grand plan

A plain markdown file the human owns. Suggested structure:

```markdown
# Grand plan — week of 2026-05-17

## Objective
Ship cc-lens-ui v0.2 by Friday.

## Streams
- **frontend** (cwd contains `cc-lens-ui`) — finish the settings panel
  polish, write screenshots, tag v0.2.0.
- **backend** (cwd contains `cc-lens`) — add `--with-tmux-rename` to
  `install-hooks`; nothing else.
- **research** (cwd contains `scratch/`) — explore IndexedDB caching;
  produce a memo, no code.

## Out of scope this week
- Any docs work other than screenshots.
- Any new dependencies.
```

The manager re-reads this file every tick. Edit it whenever your
priorities change; the next run picks it up.

## Prompt

Given to Claude (Opus, headless) once per tick — *not* once per agent.
Passing all sessions at once lets the manager see the whole portfolio
and coordinate between them (e.g., "agent A is blocked on B's output,
ping B first").

```text
You are the manager of a set of running Claude Code sessions. Your job
is to keep each one on the grand plan with the minimum possible
interference. The default is to say nothing.

THE GRAND PLAN
==============
{plan_markdown}

CURRENT SESSIONS
================
For each session, you see: id (short), cwd, host, last_kind, idle
duration, and the last 15 transcript lines (most recent last).

{sessions_block}

INSTRUCTIONS
============
For each session, choose exactly one decision and (if not `ok`) the
exact message to queue. Output strictly as JSON:

[
  {
    "session_id": "<full id>",
    "decision": "ok" | "checkin" | "redirect" | "stop",
    "message": "<text to queue, or empty string if decision is ok>",
    "reason": "<one sentence, for the human reviewing the log>"
  },
  ...
]

Rules:
- Prefer `ok`. Only escalate when you'd defend the message in a code review.
- `checkin` only if idle > 10 minutes AND the session is on a stream of
  work that should still be active. Format: "any update? reply
  `continue`, `done`, or describe the blocker."
- `redirect` when the work is real but on the wrong thing per the plan.
  Tell them the right thing in one sentence, not a paragraph.
- `stop` only for genuinely off-plan or harmful work. Suggest stopping
  and asking the human to confirm scope before continuing.
- Never send the same message text two ticks in a row to the same
  session — assume your previous tick's nudge has been ignored for a
  reason.
- If two sessions are on related work, you may coordinate (e.g.,
  redirect A to wait for B). Note the dependency in `reason`.
```

## cc-lens calls

A reference shell loop the routine wraps around. `$CC_LENS_URL` and
`$CC_LENS_TOKEN` come from the environment; `$PLAN` is the path to your
grand-plan markdown.

```bash
#!/usr/bin/env bash
set -euo pipefail
: "${CC_LENS_URL:?}" "${CC_LENS_TOKEN:?}" "${PLAN:=$HOME/.cc-lens/plan.md}"

curl_lens() {
  curl -sfH "Authorization: Bearer $CC_LENS_TOKEN" "$CC_LENS_URL$1"
}

# 1. Snapshot sessions and their recent transcripts.
sessions_json=$(curl_lens /sessions)
sessions_block=""
while read -r id cwd host last_kind last_ts; do
  transcript=$(curl_lens "/sessions/$id/transcript?limit=15" \
    | jq -r '.[] | "[\(.type)] \(.timestamp) \(.uuid[0:8])"' || echo "(empty)")
  idle_secs=$(($(date +%s) - $(date -d "$last_ts" +%s 2>/dev/null || echo 0)))
  sessions_block+="--- $id ($cwd, $host, $last_kind, idle=${idle_secs}s) ---
$transcript

"
done < <(jq -r '.[] | "\(.id) \(.cwd) \(.host) \(.last_kind) \(.last_ts)"' <<< "$sessions_json")

# 2. Ask the manager (Opus, headless) for a decision per session.
decisions=$(claude -p --model claude-opus-4-7 <<PROMPT
$(cat "$PLAN")
… [paste the Prompt block above, with {plan_markdown} and {sessions_block} interpolated] …
PROMPT
)

# 3. For each non-`ok` decision, queue the message.
echo "$decisions" | jq -c '.[] | select(.decision != "ok")' | while read -r d; do
  sid=$(jq -r .session_id <<< "$d")
  msg=$(jq -r .message <<< "$d")
  reason=$(jq -r .reason <<< "$d")
  echo "[manager] $sid: $(jq -r .decision <<< "$d") — $reason"
  curl -sfX POST "$CC_LENS_URL/agents/$sid/messages" \
    -H "Authorization: Bearer $CC_LENS_TOKEN" \
    -H "Content-Type: application/json" \
    -d "$(jq -n --arg b "$msg" '{body:$b}')" > /dev/null
done
```

## Adapting this

- **Where the plan lives.** `~/.cc-lens/plan.md` is just a default. Point
  `$PLAN` at a shared file (Dropbox, a git repo, an S3 object) if more
  than one human edits it.
- **Cadence.** 15 minutes is a starting point. A team that runs many
  short-lived sessions might tighten to 5; a solo "leave it on overnight"
  setup might relax to 30. Don't go below 2 minutes — you'll outpace the
  agents' own turn lengths and the nudges start looking schizophrenic.
- **Idle threshold for `checkin`.** 10 minutes is conservative. Lower it
  if you've trained your agents to expect frequent check-ins; raise it
  if your sessions naturally have long "thinking" pauses.
- **The bias toward silence is load-bearing.** A manager that nudges on
  every tick gets muted. Track the ratio of `ok` to non-`ok` in your log
  — if it dips below ~90%, your plan is probably too vague or your
  threshold is too tight, not that all your agents are misbehaving.
- **Audit log.** The `echo` in step 3 is the audit trail. Pipe it to a
  file with a date stamp and review it weekly — you'll find both
  hallucinated redirects (manager bug) and legitimate ones you'd have
  missed.
- **Multi-tenant.** If different users' sessions share a cc-lens, give
  each user their own plan and run one manager per plan, filtering
  sessions by `workspace` or `cwd` prefix. Don't try to put every user's
  plan in one prompt — they'll bleed.
