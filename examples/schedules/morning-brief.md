---
name: morning-brief
description: Daily 08:00 plan generator that turns TODO and routine markdown into a single brief and flags any Claude sessions still in flight.
schedule:
  cron: "0 8 * * *"
  timezone: "Europe/London"
cc_lens:
  reads:  [GET /sessions]
inputs:
  - kind: file
    description: One or more markdown files holding your TODO list and your daily routine. Paths configurable.
  - kind: file
    description: Last night's evening-wrap reply (if any), so the brief acknowledges what you said you'd do.
outputs:
  channel: messenger
  format: markdown
---

# Morning brief

Generates a short, actionable plan for the day at 08:00. Reads whatever
markdown files you point it at (a TODO list, a recurring-routine doc, last
night's wrap reply), asks cc-lens which Claude Code sessions are still
alive, and writes one message you can read on your phone over coffee.

This is a "lens" example in the weak sense: it only *reads* from cc-lens.
The brief itself is markdown delivered to a messenger — cc-lens never sees
the output and no agent state is mutated.

## Prompt

```
You are writing my morning brief for {{today}}.

Inputs:
- TODO list:           {{file:TODO.md}}
- Daily routine:       {{file:ROUTINE.md}}
- Last night's reply:  {{file:wrap-reply-yesterday.md}}   (may be empty)
- In-flight Claude sessions (from cc-lens): {{cc_lens_sessions}}

Produce a single markdown message with these sections:

1. **Top 3 for today** — pick from the TODO list, weighted toward what I
   said I'd do in last night's reply. Each item one line.
2. **Routine reminders** — only the items from ROUTINE.md that apply to
   today's weekday.
3. **Sessions still open** — for each cc-lens session whose last_ts is
   within the last 24h, one line: `<short-id> · <cwd basename> · last
   <last_kind> <relative-time>`. Skip this section entirely if there are
   none.
4. **One question for me** — the single most useful clarifying question
   you'd ask before I start.

Keep the whole thing under 200 words. Plain markdown, no preamble.
```

## cc-lens calls

```sh
# In-flight sessions, filtered to those touched in the last 24h.
curl -sH "Authorization: Bearer $CC_LENS_TOKEN" \
  "$CC_LENS_URL/sessions" \
  > /tmp/sessions.json
```

The prompt's `{{cc_lens_sessions}}` placeholder is the raw JSON body. The
model is responsible for filtering by `last_ts` — keeping the curl side
dumb means you can swap in any other client (Bun, Python, a GitHub Action)
without re-implementing the same predicate.

## Adapting this

- Point `TODO.md` and `ROUTINE.md` at whatever you actually use — Obsidian,
  a private gist, a Notes export. The example assumes plain markdown on
  disk so any runner can read it.
- Change `outputs.channel` to email or Slack if you don't have a messenger
  webhook handy. The brief is just markdown.
- If you don't run [evening-wrap](./evening-wrap.md), drop the "last
  night's reply" input and the corresponding paragraph in the prompt.
