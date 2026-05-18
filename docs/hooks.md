# Installing hooks

`cc-lens install-hooks` patches your Claude Code [`settings.json`][settings]
so every supported hook event is POSTed to your `cc-lens` server.

[settings]: https://docs.claude.com/en/docs/claude-code/settings

## What the command does

```sh
cc-lens install-hooks --server http://127.0.0.1:8787 --token hunter2
```

1. Resolves `~/.claude/settings.json` (uses `os.UserHomeDir()`, so it works on
   Linux/macOS/Windows).
2. Reads the existing JSON, preserving every key you've already set.
3. Replaces the `hooks` block with one entry per supported event kind:
   `PreToolUse`, `PostToolUse`, `UserPromptSubmit`, `Notification`, `Stop`,
   `SubagentStop`, `PreCompact`, `SessionStart`, `SessionEnd`.
4. Each entry runs:

   ```sh
   cat | jq --arg kind <KIND> --arg host <HOST> --arg workspace <WS> \
       '. + {kind:$kind, host:$host, workspace:$workspace}' \
     | curl -sS --max-time 5 -H "Authorization: Bearer ..." \
       -H "Content-Type: application/json" \
       -X POST http://.../events -d @-
   ```

5. Writes atomically (write to `.tmp`, then rename).

## Flags

| Flag           | Default                       | Purpose                                                  |
| -------------- | ----------------------------- | -------------------------------------------------------- |
| `--server`     | `http://127.0.0.1:8787`       | cc-lens base URL. The hooks POST to `${server}/events`.  |
| `--token`      | *(empty)*                     | Bearer token. Must match the server.                     |
| `--workspace`  | *(empty)*                     | Free-form label stamped into every event.                |
| `--path`       | `~/.claude/settings.json`     | Settings file to patch. Use the project-level path for a per-repo install. |

## Why you almost always want `--workspace`

The `--workspace` flag defaults to empty. When unset, `install-hooks`
bakes an empty string into the generated curl command, so every event
arrives at the server with `workspace=""`. The dashboard and
`/events?workspace=...` filter then can't tell your projects apart — they
all collapse into one bucket.

There's no environment-variable fallback inside the hook itself: the
generated command is a one-shot string baked at install time, not a
shell that re-reads `$CC_LENS_WORKSPACE` on each invocation. If you want
env-driven config, drive it at install time:

```sh
cc-lens install-hooks \
  --path ./.claude/settings.json \
  --workspace "${CC_LENS_WORKSPACE:?set this in your shell rc}" \
  --server "$CC_LENS_SERVER" --token "$CC_LENS_TOKEN"
```

Rule of thumb: install hooks per-project (see below) with an explicit
`--workspace`, rather than installing a single global hook with no label.
A global install is fine for "is anything running?" usage, but loses
information the moment you have more than one repo.

## Per-project hooks

To scope hooks to a single repository instead of your user account, point
`--path` at the project settings file:

```sh
cc-lens install-hooks \
  --path ./.claude/settings.json \
  --server http://127.0.0.1:8787 --token hunter2 \
  --workspace my-project
```

## Requirements on the Claude Code host

Each hook command pipes through `jq` and `curl`. Both are present on macOS
(via Homebrew or built-in), most Linux distros, and Windows-with-WSL. The
generated hook is Unix-shell-shaped; bare Windows PowerShell support is not
yet shipped (PRs welcome).

## Manual install (if you'd rather not let cc-lens edit your settings)

Open `~/.claude/settings.json` and add (replacing the URL and token):

```json
{
  "hooks": {
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "cat | jq --arg kind \"Stop\" --arg host \"$(hostname)\" --arg workspace \"\" '. + {kind:$kind, host:$host, workspace:$workspace}' | curl -sS --max-time 5 -H \"Authorization: Bearer hunter2\" -H \"Content-Type: application/json\" -X POST http://127.0.0.1:8787/events -d @-"
          }
        ]
      }
    ]
  }
}
```

Repeat for each kind you care about. The installer is a convenience; the
schema is plain JSON.

## Verifying the install

In one terminal:

```sh
curl -N -H "Authorization: Bearer hunter2" http://127.0.0.1:8787/stream
```

In another, start a Claude Code session and have it do anything. You should
see `event: PreToolUse`, `event: PostToolUse`, etc. land in the first
terminal within a second.

## Troubleshooting

- **No events arriving.** Run the generated hook command by hand with a fake
  payload: `echo '{}' | <the hook command>` — you should get a 202.
- **`jq: command not found`** — install jq (`brew install jq` /
  `apt install jq`).
- **`curl: (28) Operation timed out`** — cc-lens isn't reachable from the
  Claude Code host. Check `--addr` and any firewall.
- **Hook fails silently.** Claude Code surfaces hook stderr in its session log;
  run `claude --debug` for more detail.
