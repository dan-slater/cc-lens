---
name: evening-wrap
description: Daily 20:00 end-of-day check-in. Asks two questions and stores your reply for tomorrow's morning brief.
schedule:
  cron: "0 20 * * *"
  timezone: "Europe/London"
cc_lens: {}
inputs:
  - kind: file
    description: Yesterday's wrap reply, for diffing tone over time. Optional.
outputs:
  channel: messenger
  format: markdown
---

# Evening wrap

Sends two questions at 20:00 — "what shipped today?" and "what's tomorrow's
first move?" — and stores whatever you reply to a known file so the next
morning's brief can read it. Deliberately tiny: the value is the
prompt-then-capture loop, not the prompt content.

This is the "lens" rule taken to its limit: the routine doesn't touch
cc-lens at all. It is included here because it pairs directly with
[morning-brief](./morning-brief.md) and shows that "scheduled job" doesn't
have to mean "scheduled job that calls cc-lens".

## Prompt

```
End of day, {{today}}. Send me a two-question check-in. Format:

> **Today** — what did you ship or learn? (one line is fine)
> **Tomorrow** — what's the first move you want to make?

Sign off with one sentence of encouragement. No preamble, no lists.
```

## Reply capture (left to the reader)

The routine sends a message. Capturing the user's reply is intentionally
out of scope here, because every messenger needs a different webhook. Two
common shapes:

- **Messenger with a server-side webhook.** Your bot writes the reply body
  to `wrap-reply-{{today}}.md` in the same directory `morning-brief` reads
  from. One file per day, keyed by ISO date. Idempotent if the user replies
  twice — overwrite or append, your call.
- **Email reply-to.** Send the prompt as email; pipe inbound replies
  through a small `procmail`/`maildrop`/`fastmail-rule` filter into the
  same `wrap-reply-{{today}}.md` file.

Either way, morning-brief just reads the file. If it isn't there, the
brief degrades gracefully (see its prompt — the "last night's reply" line
is allowed to be empty).

## cc-lens calls

None. The routine doesn't read or write cc-lens.

## Adapting this

- If you have an existing "daily questions" habit, replace the prompt and
  keep the capture mechanism — that's the reusable bit.
- If you don't run morning-brief, the file-on-disk step has no consumer
  and you can skip it. The check-in still has value as a daily nudge.
- Skip on weekends by changing the cron to `0 20 * * 1-5`.
