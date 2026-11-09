package auth

import (
	"net/http"
)

func NoAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := ContextWithUser(r.Context(), LocalUser())
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
