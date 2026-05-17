package server

import (
	"net/http/httptest"
	"testing"
)

func TestAuthHeader(t *testing.T) {
	a := Auth{Token: "secret"}
	r := httptest.NewRequest("GET", "/x", nil)
	r.Header.Set("Authorization", "Bearer secret")
	if !a.Check(r) {
		t.Fatal("header auth should succeed")
	}
}

func TestAuthQueryParam(t *testing.T) {
	a := Auth{Token: "secret"}
	r := httptest.NewRequest("GET", "/x?token=secret", nil)
	if !a.Check(r) {
		t.Fatal("query-param auth should succeed")
	}
}

func TestAuthRejectsWrongToken(t *testing.T) {
	a := Auth{Token: "secret"}
	r := httptest.NewRequest("GET", "/x?token=nope", nil)
	if a.Check(r) {
		t.Fatal("wrong query-param token must fail")
	}
	r = httptest.NewRequest("GET", "/x", nil)
	r.Header.Set("Authorization", "Bearer nope")
	if a.Check(r) {
		t.Fatal("wrong header token must fail")
	}
}

func TestAuthEmptyTokenDisablesAuth(t *testing.T) {
	a := Auth{Token: ""}
	r := httptest.NewRequest("GET", "/x", nil)
	if !a.Check(r) {
		t.Fatal("empty token should disable auth")
	}
}
