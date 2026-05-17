---
name: session-digest
description: Daily 09:00 digest of every Claude Code session touched yesterday — counts, longest sessions, top tools, errors observed.
schedule:
  cron: "0 9 * * *"
  timezone: "Europe/London"
cc_lens:
  reads:  [GET /sessions, GET /sessions/{id}/transcript]
inputs: []
outputs:
  channel: messenger
  format: markdown
---

# Session digest

A cc-lens-native routine that produces a one-page "what did I do with
Claude Code yesterday?" digest. Pulls the session list, filters to rows
whose `modified_at` falls in yesterday's window, then pages through each
session's transcript and counts tool uses, identifies errors, and picks
the few sessions worth re-reading.

Read-only on cc-lens; the digest itself is markdown delivered to a
messenger or email. Pairs well with [morning-brief](./morning-brief.md):
the brief tells you what's *open*, the digest tells you what *happened*.

## Prompt

```
You are writing my Claude Code session digest for {{yesterday}}.

Inputs:
- Sessions touched yesterday (JSON from cc-lens): {{sessions}}
- For each session, the last 200 transcript lines (concatenated, with
  session-id headers): {{transcripts}}

Produce a markdown digest with these sections:

  ## Summary
  One line: `<n> sessions across <m> workspaces, <total transcript bytes>`.

  ## Longest 3
  For each: `<short-id>` · `<workspace>` · `<n lines>` · one-sentence
  summary of what the session was doing.

  ## Top tool uses
  Bullet list, descending. Count tool-use events from the transcript raw
  payloads (look for `"type":"tool_use"` in assistant messages).

  ## Errors observed
  Any transcript line whose `raw` contains `is_error: true` or a stderr-
  shaped payload. Group by session. One line each. If none, write
  `_None._`.

  ## Worth re-reading
  Up to 3 sessions worth a human revisit — pick by judgement, not metrics
  (high error count, abrupt end, unusual tool sequence).

Plain markdown. Under 300 words.
```

## cc-lens calls

```sh
# 1. Sessions touched yesterday.
curl -sH "Authorization: Bearer $CC_LENS_TOKEN" \
  "$CC_LENS_URL/sessions" > /tmp/sessions.json

# 2. For each one whose modified_at falls in yesterday's window,
#    page the transcript. (Cap at 200 lines per session — long enough
#    for shape, short enough to keep the context window sane.)
yesterday=$(date -u -d 'yesterday' +%F)
jq -r --arg d "$yesterday" '
  .[] | select(.modified_at != null and (.modified_at | startswith($d)))
       | .id
' /tmp/sessions.json | while read -r id; do
  curl -sH "Authorization: Bearer $CC_LENS_TOKEN" \
    "$CC_LENS_URL/sessions/$id/transcript?limit=200" \
    > "/tmp/digest-$id.json"
done

# 3. Concatenate the transcripts, with session-id headers, and feed the
#    whole bundle into the prompt.
{
  for f in /tmp/digest-*.json; do
    id=$(basename "$f" .json | sed 's/^digest-//')
    echo "=== session $id ==="
    cat "$f"
  done
} > /tmp/transcripts.txt
```

The `limit=200` on `/sessions/{id}/transcript` is the right cheap default
— enough to characterise the session, small enough that you can run the
digest over 50 sessions without hitting model context limits. If you need
deeper history use `before=<uuid>` to paginate backwards.

## Adapting this

- **Change the window.** Weekly digest? Filter `modified_at` over the
  last seven days and ask for "top sessions per day" in the prompt.
- **Pivot the metrics.** The example counts tool uses; counting `usage`
  fields from assistant messages gives a tokens-per-session view if you
  care about that.
- **Email instead of messenger.** A week-of-digests as an email thread
  is genuinely useful for after-action reviews.
