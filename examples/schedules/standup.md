---
name: standup
description: Twice-daily standup brief from a code host (GitHub, GitLab, Linear). Parameterised on flavour (morning or afternoon).
schedule:
  cron: "30 7,13 * * 1-5"
  timezone: "Europe/London"
  parameters:
    flavour: morning | afternoon   # picked from the hour: 07 → morning, 13 → afternoon
cc_lens: {}
inputs:
  - kind: github | gitlab | linear
    description: A code host you can query read-only — PRs, issues, comments since the last cutoff. Connector is left abstract; the prompt expects a JSON blob.
outputs:
  channel: messenger
  format: markdown
---

# Standup

One file, two firings. At 07:30 it asks "what's on the plate today?"; at
13:30 it asks "what moved since this morning?". The only difference between
the two runs is the `flavour` parameter and the time window the connector
queries.

Strictly read-only across the upstream code host. No comments posted, no
PRs touched. The output is a markdown brief delivered to a messenger; that
is the only side effect.

## Prompt

```
You are writing my {{flavour}} standup for {{today}}.

flavour: {{flavour}}              # "morning" or "afternoon"
window:  {{window_start}}..{{now}}
host events (JSON): {{host_events}}

If flavour == "morning":
  - List the PRs and issues that need *my* action today, grouped by repo.
  - For each: one line, ending with the single most useful next step.
  - Below that, a "watching" list of PRs awaiting CI or review from others.

If flavour == "afternoon":
  - List what changed in `window` — merged PRs, new review comments
    addressed to me, status changes on issues I own.
  - End with a single "now what?" bullet picking the highest-leverage
    follow-up.

Keep under 150 words. Plain markdown. No preamble.
```

## cc-lens calls

None. cc-lens has nothing to add here — standup is about upstream code
host state, not Claude session state. (If you want a Claude-session-flavoured
standup, see [session-digest](./session-digest.md).)

## Connector sketch

The connector is intentionally abstract. The prompt expects a JSON blob
called `host_events`; whatever you stuff in there is what the model
reasons about. Two starting points:

```sh
# GitHub: PRs and issues touching you since the window start
gh api graphql -F query=@queries/standup.graphql \
  -F since="$WINDOW_START" \
  -F login="$GITHUB_LOGIN" \
  > /tmp/host_events.json
```

```sh
# Linear: issues assigned to you, updated since the window start
curl -sH "Authorization: $LINEAR_API_KEY" -H "Content-Type: application/json" \
  -d "{\"query\":\"{ issues(filter:{assignee:{isMe:{eq:true}}, updatedAt:{gte:\\\"$WINDOW_START\\\"}}) { nodes { id title state { name } url } } }\"}" \
  https://api.linear.app/graphql \
  > /tmp/host_events.json
```

## Adapting this

- Pick one connector. Mixing two in one routine is possible but the model
  gets noisier — easier to run two parallel routines that share the prompt.
- The `flavour` switch is a `case` in your wrapper script — keyed off
  `date +%H` or passed as `$1`. Don't fork the markdown.
- Skip Mondays' morning fire if your weekly planning happens elsewhere
  (change the cron to `30 7 * * 2-5,30 13 * * 1-5`).
