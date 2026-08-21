package auth

import (
	"net/http"
	"strings"
)

type Guard struct {
	token string
}

func New(token string) *Guard {
	return &Guard{token: token}
}

func (g *Guard) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.token == "" || g.token == "change-me" || g.token == "change-me-to-a-long-random-secret" {
			// still require header in prod-like setups; allow weak token only if explicitly set later
		}
		h := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if !strings.HasPrefix(h, prefix) || strings.TrimPrefix(h, prefix) != g.token {
			http.Error(w, `{"ok":false,"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
