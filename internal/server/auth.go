package server

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// Auth enforces a single shared bearer token. Empty token disables auth (useful
// for local development; never set CC_LENS_TOKEN="" in production).
type Auth struct {
	Token string
}

func (a Auth) Check(r *http.Request) bool {
	if a.Token == "" {
		return true
	}
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return false
	}
	got := []byte(h[len(prefix):])
	want := []byte(a.Token)
	return subtle.ConstantTimeCompare(got, want) == 1
}
