package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Config controls server behaviour. Zero values are sensible defaults.
type Config struct {
	Addr         string
	Token        string
	RingSize     int
	WebhookURL   string
	WebhookKinds string
}

type Server struct {
	cfg     Config
	auth    Auth
	bus     *Bus
	queue   *Queue
	webhook *Webhook
}

func New(cfg Config) *Server {
	if cfg.Addr == "" {
		cfg.Addr = ":8787"
	}
	if cfg.RingSize <= 0 {
		cfg.RingSize = 1000
	}
	s := &Server{
		cfg:     cfg,
		auth:    Auth{Token: cfg.Token},
		bus:     NewBus(cfg.RingSize),
		queue:   NewQueue(),
		webhook: NewWebhook(cfg.WebhookURL, cfg.WebhookKinds),
	}
	return s
}

// Handler returns an http.Handler that can be served by anything.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /events", s.guarded(s.postEvents))
	mux.HandleFunc("GET /events", s.guarded(s.getEvents))
	mux.HandleFunc("GET /sessions", s.guarded(s.getSessions))
	mux.HandleFunc("GET /sessions/{id}", s.guarded(s.getSession))
	mux.HandleFunc("GET /sessions/{id}/transcript", s.guarded(s.getTranscript))
	mux.HandleFunc("GET /stream", s.guarded(s.getStream))
	mux.HandleFunc("POST /agents/{id}/messages", s.guarded(s.postMessage))
	mux.HandleFunc("GET /agents/{id}/messages", s.guarded(s.getMessages))
	mux.HandleFunc("POST /messages/{id}/ack", s.guarded(s.ackMessage))
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func (s *Server) ListenAndServe() error {
	srv := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	return srv.ListenAndServe()
}

// guarded wraps a handler with bearer-token auth.
func (s *Server) guarded(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.Check(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h(w, r)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// ---- handlers ----

func (s *Server) postEvents(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1 MiB hard cap
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var probe struct {
		SessionID string `json:"session_id"`
		Kind      string `json:"kind"`
		CWD       string `json:"cwd"`
		Workspace string `json:"workspace"`
		Host      string `json:"host"`
		Label     string `json:"label"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if probe.Kind == "" {
		http.Error(w, "kind required", http.StatusBadRequest)
		return
	}
	e := s.bus.Publish(Event{
		SessionID: probe.SessionID,
		Kind:      probe.Kind,
		CWD:       probe.CWD,
		Workspace: probe.Workspace,
		Host:      probe.Host,
		Label:     probe.Label,
		Raw:       body,
	})
	if s.webhook.Match(e.Kind) {
		s.webhook.Fire(context.Background(), e)
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"id": e.ID})
}

func (s *Server) getEvents(w http.ResponseWriter, r *http.Request) {
	sid := r.URL.Query().Get("session_id")
	since, _ := strconv.ParseUint(r.URL.Query().Get("since_id"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	writeJSON(w, http.StatusOK, s.bus.Snapshot(sid, since, limit))
}

// SessionRow merges the live (hook) and on-disk views.
type SessionRow struct {
	ID              string    `json:"id"`
	LastKind        string    `json:"last_kind,omitempty"`
	LastTS          time.Time `json:"last_ts,omitempty"`
	CWD             string    `json:"cwd,omitempty"`
	Workspace       string    `json:"workspace,omitempty"`
	Host            string    `json:"host,omitempty"`
	Label           string    `json:"label,omitempty"`
	TranscriptPath  string    `json:"transcript_path,omitempty"`
	TranscriptBytes int64     `json:"transcript_bytes,omitempty"`
	ModifiedAt      time.Time `json:"modified_at,omitempty"`
}

func (s *Server) getSessions(w http.ResponseWriter, r *http.Request) {
	rows := map[string]*SessionRow{}
	for sid, e := range s.bus.LatestBySession() {
		rows[sid] = &SessionRow{
			ID:        sid,
			LastKind:  e.Kind,
			LastTS:    e.ReceivedAt,
			CWD:       e.CWD,
			Workspace: e.Workspace,
			Host:      e.Host,
			Label:     e.Label,
		}
	}
	disk, _ := DiscoverSessions()
	for _, d := range disk {
		row, ok := rows[d.ID]
		if !ok {
			row = &SessionRow{ID: d.ID}
			rows[d.ID] = row
		}
		row.TranscriptPath = d.TranscriptPath
		row.TranscriptBytes = d.TranscriptBytes
		row.ModifiedAt = d.ModifiedAt
		if row.CWD == "" {
			row.CWD = d.CWD
		}
	}
	out := make([]*SessionRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, r)
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) getSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	rows := map[string]*SessionRow{}
	if e, ok := s.bus.LatestBySession()[id]; ok {
		rows[id] = &SessionRow{
			ID: id, LastKind: e.Kind, LastTS: e.ReceivedAt, CWD: e.CWD,
			Workspace: e.Workspace, Host: e.Host, Label: e.Label,
		}
	}
	disk, _ := DiscoverSessions()
	for _, d := range disk {
		if d.ID != id {
			continue
		}
		row, ok := rows[id]
		if !ok {
			row = &SessionRow{ID: id}
			rows[id] = row
		}
		row.TranscriptPath = d.TranscriptPath
		row.TranscriptBytes = d.TranscriptBytes
		row.ModifiedAt = d.ModifiedAt
		if row.CWD == "" {
			row.CWD = d.CWD
		}
	}
	row, ok := rows[id]
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, row)
}

func (s *Server) getTranscript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	disk, _ := DiscoverSessions()
	var path string
	for _, d := range disk {
		if d.ID == id {
			path = d.TranscriptPath
			break
		}
	}
	if path == "" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	before := r.URL.Query().Get("before")
	lines, err := ReadTranscript(path, limit, before)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, lines)
}

func (s *Server) getStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := s.bus.Subscribe()
	defer s.bus.Unsubscribe(ch)
	keepalive := time.NewTicker(20 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-keepalive.C:
			_, _ = fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case e, ok := <-ch:
			if !ok {
				return
			}
			b, _ := json.Marshal(e)
			_, _ = fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", e.ID, sseEventName(e.Kind), b)
			flusher.Flush()
		}
	}
}

func sseEventName(kind string) string {
	if kind == "" {
		return "event"
	}
	return strings.ReplaceAll(kind, " ", "_")
}

func (s *Server) postMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var body struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if body.Body == "" {
		http.Error(w, "body required", http.StatusBadRequest)
		return
	}
	m := s.queue.Enqueue(id, body.Body)
	writeJSON(w, http.StatusAccepted, m)
}

func (s *Server) getMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	writeJSON(w, http.StatusOK, s.queue.Pending(id))
}

func (s *Server) ackMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !s.queue.Ack(id) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
