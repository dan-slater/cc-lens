package cmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// TmuxRelay polls cc-lens for pending messages and delivers them to tmux
// panes named "claude:<session_id>" via `tmux send-keys`.
func TmuxRelay(args []string) error {
	fs := flag.NewFlagSet("tmux-relay", flag.ExitOnError)
	server := fs.String("server", "http://127.0.0.1:8787", "cc-lens server URL")
	token := fs.String("token", "", "bearer token")
	interval := fs.Duration("interval", 2*time.Second, "poll interval")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		return fmt.Errorf("tmux not found in PATH: %w", err)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	t := time.NewTicker(*interval)
	defer t.Stop()
	for range t.C {
		ids, err := listAgentIDs(client, *server, *token)
		if err != nil {
			fmt.Println("relay: list:", err)
			continue
		}
		for _, id := range ids {
			msgs, err := drainAgent(client, *server, *token, id)
			if err != nil {
				fmt.Println("relay: drain:", err)
				continue
			}
			for _, m := range msgs {
				target := "claude:" + id
				if err := exec.Command("tmux", "send-keys", "-t", target, m.Body, "Enter").Run(); err != nil {
					fmt.Printf("relay: tmux send-keys to %s failed: %v\n", target, err)
					continue
				}
				_ = ack(client, *server, *token, m.ID)
			}
		}
	}
	return nil
}

type relayMessage struct {
	ID      string `json:"id"`
	AgentID string `json:"agent_id"`
	Body    string `json:"body"`
}

type relaySession struct {
	ID string `json:"id"`
}

func listAgentIDs(c *http.Client, server, token string) ([]string, error) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server+"/sessions", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var rows []relaySession
	if err := json.Unmarshal(body, &rows); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out, nil
}

func drainAgent(c *http.Client, server, token, id string) ([]relayMessage, error) {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, server+"/agents/"+id+"/messages", nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var msgs []relayMessage
	if err := json.Unmarshal(body, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

func ack(c *http.Client, server, token, id string) error {
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, server+"/messages/"+id+"/ack", strings.NewReader(""))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
