---
name: doc-drift-report
description: Nightly read-only scan of a documentation tree. Reports staleness, duplication, and broken links. Never writes.
schedule:
  cron: "0 3 * * *"
  timezone: "Europe/London"
cc_lens: {}
inputs:
  - kind: file
    description: A directory containing markdown documentation. Configurable; the example assumes `docs/`.
outputs:
  channel: messenger
  format: markdown
---

# Doc drift report

A nightly read-only walk over a docs tree, producing a single markdown
report covering three questions:

1. **What looks stale?** Files whose `mtime` predates the most-recent
   change to the code they describe (heuristic: nearest `*.go`/`*.ts`
   under a shared ancestor). Also flags any "as of <date>" sentences whose
   date is more than 90 days old.
2. **What looks duplicated?** Paragraphs whose normalised text appears in
   more than one file. The model picks the canonical home and lists the
   echo sites.
3. **What links are broken?** Anchor-existence check for in-repo links;
   HEAD request for off-repo links (timeout 5s, treat 401/403 as "ok",
   only flag 4xx/5xx that aren't auth).

## Redesigned from the orchestrator's doc-agent

The original `claude-orchestrator/bin/doc-agent.ts` was authorised to
*mutate* git: it would rewrite files it judged stale, open branches, and
sometimes commit. This version deliberately removes that. It only
produces a report. Acting on the report is the human's job (or another
routine you write yourself, with eyes on it).

The reason isn't doctrinal; it's empirical. The mutating version was
high-throughput and roughly half its commits were wrong in ways that
were time-consuming to revert. A reporting-only routine has roughly the
same time-to-signal but zero clean-up cost, and is therefore the version
that earns its keep on a recurring schedule.

This is the strongest "lens" example: the routine never writes anywhere
except the messenger output channel. cc-lens is not involved at all.

## Prompt

```
You are auditing the docs tree at {{docs_root}}.

Inputs:
- A list of all markdown files under {{docs_root}} with their mtimes:
  {{files_with_mtimes}}
- The last 30 days of git log touching the surrounding code tree:
  {{code_changes}}
- A flat list of every (link_target, source_file, line_number) tuple:
  {{links}}
- HEAD-check results for external links: {{link_status}}

Produce a markdown report with three sections:

  ## Stale
  Files where the doc mtime is older than the youngest code change in the
  same subtree. For each: `<path>` — last touched <date>, but
  `<code_path>` changed <date>. One line per file.

  ## Duplicated
  Paragraphs whose first 200 chars appear in more than one file. Pick a
  canonical home (the file whose mtime is oldest), list the echo sites.

  ## Broken links
  Group by source file. For each broken link: target, line, reason.

If a section is empty, write `_None._` and move on. No preamble.
```

## cc-lens calls

None. This routine doesn't touch cc-lens.

## Adapting this

- Point `{{docs_root}}` at any markdown tree. The "nearest code" heuristic
  is easy to fold in for monorepos; for a docs-only repo, drop the stale
  check and rely on the "as of" date scan.
- If you want to act on the report, send it to yourself and triage by
  hand — or feed it as input to a *separate* routine that opens branches
  with you in the review loop. Keep the mutating step out of the cron.
- External link-checking is the slowest step. Cache HEAD responses for a
  week (`mkdir -p .linkcache && curl -I --output .linkcache/$(echo $url | sha1sum)`).
