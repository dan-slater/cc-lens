package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// Webhook fans out events matching Kinds (empty = all) to URL. Fire-and-forget
// with one retry; failures are logged and dropped.
type Webhook struct {
	URL    string
	Kinds  map[string]bool
	Client *http.Client
}

func NewWebhook(url, kindsCSV string) *Webhook {
	w := &Webhook{
		URL:    url,
		Kinds:  map[string]bool{},
		Client: &http.Client{Timeout: 5 * time.Second},
	}
	for _, k := range strings.Split(kindsCSV, ",") {
		k = strings.TrimSpace(k)
		if k != "" {
			w.Kinds[k] = true
		}
	}
	return w
}

func (w *Webhook) Match(kind string) bool {
	if len(w.Kinds) == 0 {
		return true
	}
	return w.Kinds[kind]
}

func (w *Webhook) Fire(ctx context.Context, e Event) {
	if w == nil || w.URL == "" {
		return
	}
	body, err := json.Marshal(e)
	if err != nil {
		return
	}
	go func() {
		for attempt := 0; attempt < 2; attempt++ {
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, bytes.NewReader(body))
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := w.Client.Do(req)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode < 500 {
					return
				}
			}
			time.Sleep(500 * time.Millisecond)
		}
		log.Printf("webhook: gave up delivering event %d to %s", e.ID, w.URL)
	}()
}
