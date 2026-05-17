package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// InstallHooks writes a hooks block to ~/.claude/settings.json that POSTs
// every supported event to cc-lens.
func InstallHooks(args []string) error {
	fs := flag.NewFlagSet("install-hooks", flag.ExitOnError)
	server := fs.String("server", "http://127.0.0.1:8787", "cc-lens server URL")
	token := fs.String("token", "", "bearer token (matches server --token)")
	workspace := fs.String("workspace", "", "workspace label included with every event")
	path := fs.String("path", defaultSettingsPath(), "settings.json path")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cur, err := readSettings(*path)
	if err != nil {
		return err
	}
	host, _ := os.Hostname()
	hooks := buildHooks(*server, *token, *workspace, host)
	cur["hooks"] = hooks
	return writeSettings(*path, cur)
}

func defaultSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".claude", "settings.json")
}

func readSettings(path string) (map[string]any, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func writeSettings(path string, m map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// buildHooks emits the cross-platform curl invocation Claude Code will run on
// each hook event. We use --max-time so a hung cc-lens cannot freeze the agent.
func buildHooks(server, token, workspace, host string) map[string]any {
	kinds := []string{
		"PreToolUse", "PostToolUse", "UserPromptSubmit", "Notification",
		"Stop", "SubagentStop", "PreCompact", "SessionStart", "SessionEnd",
	}
	authHeader := ""
	if token != "" {
		authHeader = fmt.Sprintf(`-H "Authorization: Bearer %s" `, token)
	}
	tmpl := `cat | jq --arg kind %q --arg host %q --arg workspace %q '. + {kind:$kind, host:$host, workspace:$workspace}' | curl -sS --max-time 5 %s-H "Content-Type: application/json" -X POST %s/events -d @-`
	out := map[string]any{}
	for _, k := range kinds {
		cmd := fmt.Sprintf(tmpl, k, host, workspace, authHeader, server)
		out[k] = []map[string]any{
			{
				"hooks": []map[string]any{
					{"type": "command", "command": cmd},
				},
			},
		}
	}
	return out
}
