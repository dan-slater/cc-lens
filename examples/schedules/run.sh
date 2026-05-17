#!/usr/bin/env bash
# run.sh — parse a schedule example file and print the curl/cron commands
# it implies. Does NOT execute anything; it just prints. The intent is to
# give you a copy-pasteable starting point for whichever runner you use.
#
# Portability: only awk, sed, grep. No jq, no yq. Works on macOS and any
# reasonable Linux.
#
# Usage:
#   ./run.sh morning-brief.md
#   ./run.sh manager.md
#
# Output sections:
#   1. Parsed frontmatter (name, cron, timezone, channel).
#   2. The exact curl snippets pulled out of the markdown's `## cc-lens calls`
#      block (between the fenced ```sh and ``` lines).
#   3. An Anthropic /schedule registration hint, ready to paste into Claude.

set -euo pipefail

usage() {
  cat <<'EOF'
run.sh — print the commands a schedule example implies.

USAGE
  run.sh <example.md>

WHAT IT DOES
  - Parses the YAML frontmatter (between the first two `---` lines).
  - Extracts the fenced `sh` block under `## cc-lens calls`, if any.
  - Prints an /schedule registration hint with the cron and prompt.

WHAT IT DOES NOT DO
  - Run the curl commands. Copy them yourself once you've reviewed them.
  - Register the routine with Anthropic. Same reason.

ENVIRONMENT
  Set CC_LENS_URL and CC_LENS_TOKEN in your shell before pasting the
  printed commands; they're left as literal $vars in the output.
EOF
}

# --- argument handling -------------------------------------------------------

case "${1:-}" in
  -h|--help|"") usage; exit 0 ;;
esac

file="$1"
[ -f "$file" ] || { echo "run.sh: no such file: $file" >&2; exit 1; }

# --- frontmatter parsing -----------------------------------------------------
# The frontmatter sits between the first two `---` lines. Extract it, then
# fish out top-level scalars with grep. Nested keys (cc_lens.reads, etc.) are
# left for the reader to inspect; the helper prints the raw block too.

frontmatter=$(awk '
  /^---[[:space:]]*$/ { hits++; if (hits == 2) exit; next }
  hits == 1 { print }
' "$file")

scalar() {
  # Pull a top-level "key: value" from the frontmatter. Returns empty if
  # the key lives under a nested mapping (which is fine for our purposes).
  echo "$frontmatter" | grep -E "^${1}:" | head -1 | sed -E "s/^${1}:[[:space:]]*//;s/^\"(.*)\"$/\\1/"
}

cron=$(echo "$frontmatter" | awk '/^[[:space:]]*cron:/ {sub(/^[[:space:]]*cron:[[:space:]]*/,""); gsub(/^"|"$/,""); print; exit}')
tz=$(echo "$frontmatter"   | awk '/^[[:space:]]*timezone:/ {sub(/^[[:space:]]*timezone:[[:space:]]*/,""); gsub(/^"|"$/,""); print; exit}')

# --- output: frontmatter summary --------------------------------------------

echo "# $(scalar name) — $(scalar description)"
echo "# cron:     ${cron:-<none>}"
echo "# timezone: ${tz:-<none>}"
echo

# --- output: cc-lens curl snippets ------------------------------------------
# Pull the first fenced ```sh block under `## cc-lens calls`. If the example
# has no cc-lens calls (it says "None."), we print nothing for this section.

calls=$(awk '
  /^## cc-lens calls/ { in_section = 1; next }
  in_section && /^## / { exit }
  in_section && /^```sh/ { in_code = 1; next }
  in_section && in_code && /^```/ { in_code = 0; next }
  in_section && in_code { print }
' "$file")

if [ -n "$calls" ]; then
  echo "# --- cc-lens calls (set CC_LENS_URL and CC_LENS_TOKEN first) ---"
  echo "$calls"
  echo
fi

# --- output: /schedule registration hint ------------------------------------

prompt=$(awk '
  /^## Prompt/ { in_section = 1; next }
  in_section && /^## / { exit }
  in_section && /^```/ { in_code = !in_code; next }
  in_section && in_code { print }
' "$file")

cat <<EOF
# --- Anthropic /schedule registration hint ---
# Paste this into Claude:
#
#   /schedule
#   cron:     ${cron:-<set me>}
#   timezone: ${tz:-<set me>}
#   prompt: |
$(echo "$prompt" | sed 's/^/#     /')
EOF
