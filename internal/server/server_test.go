package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestServer(t *testing.T) (*httptest.Server, *Server) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("CC_LENS_PROJECTS_DIR", dir)
	s := New(Config{Token: "secret"})
	srv := httptest.NewServer(s.Handler())
	t.Cleanup(srv.Close)
	return srv, s
}

func do(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, url, rdr)
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestPostEventRequiresAuth(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Post(srv.URL+"/events", "application/json", strings.NewReader(`{"kind":"K"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %d", resp.StatusCode)
	}
}

func TestPostEventRejectsMissingKind(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := do(t, "POST", srv.URL+"/events", map[string]any{"session_id": "s"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", resp.StatusCode)
	}
}

func TestPostEventAndGet(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := do(t, "POST", srv.URL+"/events", map[string]any{"kind": "Stop", "session_id": "abc"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("want 202, got %d", resp.StatusCode)
	}
	resp = do(t, "GET", srv.URL+"/events?session_id=abc", nil)
	b, _ := io.ReadAll(resp.Body)
	var got []Event
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Kind != "Stop" {
		t.Fatalf("unexpected events: %s", b)
	}
}

func TestSessionsMergesLiveAndDisk(t *testing.T) {
	srv, _ := newTestServer(t)
	// Seed a fake transcript on disk.
	root := os.Getenv("CC_LENS_PROJECTS_DIR")
	projDir := filepath.Join(root, "-Users-ds-foo")
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tpath := filepath.Join(projDir, "sess-on-disk.jsonl")
	if err := os.WriteFile(tpath, []byte(`{"type":"user","uuid":"u1","sessionId":"sess-on-disk"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Publish a live event for a different session.
	do(t, "POST", srv.URL+"/events", map[string]any{"kind": "Notification", "session_id": "sess-live"})
	resp := do(t, "GET", srv.URL+"/sessions", nil)
	b, _ := io.ReadAll(resp.Body)
	var rows []SessionRow
	if err := json.Unmarshal(b, &rows); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, r := range rows {
		seen[r.ID] = true
	}
	if !seen["sess-on-disk"] || !seen["sess-live"] {
		t.Fatalf("expected both sessions to appear, got %+v", rows)
	}
}

func TestTranscriptEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	root := os.Getenv("CC_LENS_PROJECTS_DIR")
	projDir := filepath.Join(root, "proj")
	_ = os.MkdirAll(projDir, 0o755)
	tpath := filepath.Join(projDir, "S1.jsonl")
	lines := []string{
		`{"type":"user","uuid":"u1"}`,
		`{"type":"assistant","uuid":"u2","parentUuid":"u1"}`,
		`{"type":"user","uuid":"u3","parentUuid":"u2"}`,
	}
	_ = os.WriteFile(tpath, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	resp := do(t, "GET", srv.URL+"/sessions/S1/transcript?limit=2", nil)
	b, _ := io.ReadAll(resp.Body)
	var got []TranscriptLine
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].UUID != "u3" {
		t.Fatalf("unexpected transcript: %+v", got)
	}
}

func TestMessageQueueRoundTrip(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := do(t, "POST", srv.URL+"/agents/a1/messages", map[string]any{"body": "hi"})
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("enqueue: %d", resp.StatusCode)
	}
	b, _ := io.ReadAll(resp.Body)
	var m struct{ ID string }
	_ = json.Unmarshal(b, &m)

	resp = do(t, "GET", srv.URL+"/agents/a1/messages", nil)
	b, _ = io.ReadAll(resp.Body)
	if !strings.Contains(string(b), `"body":"hi"`) {
		t.Fatalf("expected pending message in %s", b)
	}
	resp = do(t, "POST", srv.URL+"/messages/"+m.ID+"/ack", nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("ack: %d", resp.StatusCode)
	}
	resp = do(t, "GET", srv.URL+"/agents/a1/messages", nil)
	b, _ = io.ReadAll(resp.Body)
	if strings.TrimSpace(string(b)) != "[]" {
		t.Fatalf("after ack expected [], got %s", b)
	}
}

func TestHealthz(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz: %d", resp.StatusCode)
	}
}
