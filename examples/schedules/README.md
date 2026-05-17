# Scheduled-job examples

This directory collects routines that pair well with cc-lens: cron-shaped
jobs that read from cc-lens (and sometimes write a queued message back into
a running Claude Code session) on a fixed schedule.

## These are examples, not features.

Nothing here is shipped in the `cc-lens` binary. Every file is a recipe you
copy, adapt, and run from wherever you already schedule things. The
maintained branch for this content is the long-lived `examples` branch — it
is intentionally **not** merged back to `main`, because the core server has
no business growing a job runner.

## The "lens, not store" rule, applied to schedules

cc-lens reads Claude Code sessions and forwards hook events. It deliberately
owns no source of truth besides a small in-memory message queue. The
scheduled jobs here follow the same discipline:

- **Read-only across upstream APIs by default.** A standup brief that
  queries GitHub or Linear must never write back.
- **Writes, when they happen, go through cc-lens's message queue.**
  `POST /agents/{id}/messages` is the one acceptable side-effect surface,
  because the agent itself decides whether to act on the nudge. See
  [messages.md](https://github.com/dan-slater/cc-lens/blob/main/docs/messages.md).
- **No mutation of git, filesystems, or third-party state from a routine.**
  If a routine produces something durable, it produces a markdown file in
  a directory the human owns.

The two cc-lens-native examples
([manager](./manager.md) and
[session-digest](./session-digest.md)) are the ones that lean hardest on
cc-lens itself. Everything else uses cc-lens only as one input among many.

## Three ways to actually run one

Pick whichever matches the rest of your stack.

### a. Anthropic `/schedule` routine

Open Claude, run `/schedule`, paste the **Prompt** block from the example
file, and attach the **cc-lens calls** as bash steps the routine should run
first. Best when you want the routine to live inside Claude itself and
inherit your existing model + auth setup. No infra to maintain.

### b. systemd timer on the droplet

A short shell or Bun wrapper that:

1. Runs the `curl` snippets to collect inputs.
2. Pipes the prompt through `claude -p` (headless mode).
3. Posts the result to whatever messenger/email/Slack endpoint you use.

Triggered by a `.timer` unit with the cron expression from the frontmatter.
This is what runs in production for the maintainer's own setup. Pair with
the cc-lens systemd unit from
[install.md](https://github.com/dan-slater/cc-lens/blob/main/docs/install.md).

### c. GitHub Actions cron

```yaml
on:
  schedule:
    - cron: "0 8 * * *"
jobs:
  brief:
    runs-on: ubuntu-latest
    steps:
      - run: curl -sH "Authorization: Bearer $CC_LENS_TOKEN" "$CC_LENS_URL/sessions"
      - run: echo "$PROMPT" | claude -p > brief.md
        env:
          ANTHROPIC_API_KEY: ${{ secrets.ANTHROPIC_API_KEY }}
```

Best when you want zero servers of your own and are happy with GitHub's
clock. Note that cc-lens must be reachable from GitHub-hosted runners — see
the reachability note below.

## Reachability requirement

Whichever runner you pick must be able to reach `$CC_LENS_URL`. The
maintainer's setup is: cc-lens bound to `127.0.0.1:8787` on a DigitalOcean
droplet, fronted by Caddy with a public hostname and a bearer token. The
droplet/systemd recipe in
[install.md](https://github.com/dan-slater/cc-lens/blob/main/docs/install.md)
covers it end-to-end. If your runner lives off the droplet (option a or c
above) you need TLS and a token. If it lives on the droplet (option b), a
loopback bind is fine.

## Index

| File | Cadence | cc-lens role |
| --- | --- | --- |
| [morning-brief.md](./morning-brief.md) | Daily 08:00 | read `/sessions` for in-flight work |
| [evening-wrap.md](./evening-wrap.md) | Daily 20:00 | none directly; captures user reply for next morning |
| [standup.md](./standup.md) | 07:30 and 13:30 | none directly; read-only across code host |
| [meeting-notes.md](./meeting-notes.md) | 09/10/15/16 Mon–Fri | none — included as a complementary pattern |
| [doc-drift-report.md](./doc-drift-report.md) | Nightly 03:00 | none — read-only over a docs tree |
| [manager.md](./manager.md) | Every 15 min | read `/sessions` and transcripts, write `/agents/{id}/messages` — LLM-driven supervisor that keeps every session on the grand plan |
| [session-digest.md](./session-digest.md) | Daily 09:00 | read `/sessions` and `/sessions/{id}/transcript` |

Plus [run.sh](./run.sh) — a small portable helper that parses any of these
files and prints the curl commands it implies.
