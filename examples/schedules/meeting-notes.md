---
name: meeting-notes
description: Poll a Gmail label for new meeting-notes emails, write one markdown file per email under a configurable directory, send a daily extract.
schedule:
  cron: "0 9,10,15,16 * * 1-5"
  timezone: "Europe/London"
cc_lens: {}
inputs:
  - kind: gmail
    description: A Gmail label (e.g. `meeting-notes/inbox`) that you (or a Zapier/Make rule) tag emails into.
  - kind: file
    description: A target directory on disk for the extracted markdown notes.
outputs:
  channel: messenger
  format: markdown
---

# Meeting notes

Runs at four weekday slots that line up with typical meeting boundaries
(09:00, 10:00, 15:00, 16:00). On each run it queries a Gmail label for
new threads, extracts the body of each, normalises it to markdown, and
writes one file per thread under a directory you own. Once per day (the
last run wins) it also sends a digest message naming the new files.

Idempotency is by filename: `YYYY-MM-DD--<gmail-thread-id>.md`. Re-running
the routine never produces duplicates because the same thread id maps to
the same path.

cc-lens isn't involved. The example is included because the pattern —
poll a tagged inbox, write one file per item, defer interpretation to a
human — is reused all over the place and complements the other examples
when you wire them together.

## Prompt

```
You are normalising meeting notes from Gmail into markdown.

For each thread in the input, write a markdown document with this shape:

  # {{subject}}

  - **Date:** {{date}}
  - **Thread:** {{gmail_link}}
  - **Participants:** {{from}}, {{to}}

  ## Summary
  Two or three sentences. Decisions first, then context.

  ## Action items
  - [ ] One bullet per actionable item. Owner in **bold** if named.

  ## Raw
  > original email body, blockquoted

Return one document per thread, separated by a `---` line. Do not invent
action items that aren't in the source.
```

## cc-lens calls

None. This routine doesn't touch cc-lens.

## Gmail polling sketch

```sh
# Pseudocode — your Gmail client of choice. The maintainer uses an MCP
# tool but the same shape works with the official Gmail REST API.
threads=$(gmail-list --label "meeting-notes/inbox" --since "$LAST_RUN")

for tid in $threads; do
  out="$NOTES_DIR/$(date +%F)--$tid.md"
  [ -f "$out" ] && continue           # idempotency by filename
  gmail-get-thread "$tid" \
    | claude -p "$PROMPT" \
    > "$out"
done
```

The daily digest is one extra step at the 16:00 firing: list files
created today under `$NOTES_DIR`, format as a bulleted list of titles +
links, send to messenger.

## Adapting this

- Replace the Gmail polling with whatever inbox you actually use — IMAP,
  Fastmail, Superhuman API. The "tag the source, write one file per item"
  shape is the reusable bit.
- Change the slot list. Four firings per weekday is generous; one at 17:00
  works fine if your meetings cluster late.
- If you want the notes searchable, point `$NOTES_DIR` at an Obsidian
  vault or a private repo. The routine doesn't care.
