package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNoAuthMiddleware(t *testing.T) {
	var seen User
	var ok bool
	called := 0

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, ok = UserFromContext(r.Context())
		called++
	})

	handler := NoAuth(next)
	request := httptest.NewRequest("GET", "https://example.com", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, request)

	if called != 1 {
		t.Fatalf("next hanlder called %d times, want 1", called)
	}

	if !ok {
		t.Fatalf("next hanlder found no user in the request context, want one")
	}

	if seen != LocalUser() {
		t.Errorf("next handler saw user %q, want %q", seen, LocalUser())
	}

}
