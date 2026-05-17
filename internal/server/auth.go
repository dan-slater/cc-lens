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

// Check accepts the token via either the Authorization header
// ("Authorization: Bearer <token>") or a "token" query parameter. The
// query-param form exists because browser EventSource cannot set custom
// headers; it is otherwise equivalent — same secret, same blast radius —
// but be aware that URL params can land in proxy logs.
func (a Auth) Check(r *http.Request) bool {
	if a.Token == "" {
		return true
	}
	want := []byte(a.Token)
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if strings.HasPrefix(h, prefix) {
		got := []byte(h[len(prefix):])
		if subtle.ConstantTimeCompare(got, want) == 1 {
			return true
		}
	}
	if q := r.URL.Query().Get("token"); q != "" {
		if subtle.ConstantTimeCompare([]byte(q), want) == 1 {
			return true
		}
	}
	return false
}
