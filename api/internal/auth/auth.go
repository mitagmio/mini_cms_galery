package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"

	"sheyanova.art/api/internal/httpx"
)

// CookieName is the HttpOnly session cookie used for browser preview/media.
const CookieName = "sheyanova_admin"

const (
	sessionMACMsg = "sheyanova-admin-session-v1"
	cookieMaxAge  = 7 * 24 * 60 * 60
)

type Guard struct {
	token string
}

func New(token string) *Guard {
	return &Guard{token: token}
}

// Middleware requires Authorization: Bearer ADMIN_TOKEN (admin JSON API).
func (g *Guard) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !g.validBearer(r) {
			writeUnauthorized(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// BearerOrCookie allows Bearer ADMIN_TOKEN or the HttpOnly session cookie.
// Used for /preview/, /media/, and draft theme assets.
func (g *Guard) BearerOrCookie(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !g.Authorized(r) {
			writeUnauthorized(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (g *Guard) Authorized(r *http.Request) bool {
	return g.validBearer(r) || g.validCookie(r)
}

func (g *Guard) validBearer(r *http.Request) bool {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return false
	}
	got := strings.TrimPrefix(h, prefix)
	if g.token == "" || got == "" {
		return false
	}
	if len(got) != len(g.token) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(g.token)) == 1
}

func (g *Guard) validCookie(r *http.Request) bool {
	c, err := r.Cookie(CookieName)
	if err != nil || c == nil || c.Value == "" {
		return false
	}
	want := g.sessionMAC()
	if len(c.Value) != len(want) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(c.Value), []byte(want)) == 1
}

func (g *Guard) sessionMAC() string {
	mac := hmac.New(sha256.New, []byte(g.token))
	_, _ = mac.Write([]byte(sessionMACMsg))
	return hex.EncodeToString(mac.Sum(nil))
}

func isHTTPS(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	proto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if i := strings.IndexByte(proto, ','); i >= 0 {
		proto = proto[:i]
	}
	return strings.EqualFold(strings.TrimSpace(proto), "https")
}

func (g *Guard) sessionCookie(r *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     CookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   isHTTPS(r),
		SameSite: http.SameSiteLaxMode,
	}
}

func (g *Guard) SetSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, g.sessionCookie(r, g.sessionMAC(), cookieMaxAge))
}

func (g *Guard) ClearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, g.sessionCookie(r, "", -1))
}

// HandleSession: POST sets the HttpOnly cookie (Bearer required); DELETE clears it.
func (g *Guard) HandleSession(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if !g.validBearer(r) {
			writeUnauthorized(w, r)
			return
		}
		g.SetSessionCookie(w, r)
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	case http.MethodDelete:
		g.ClearSessionCookie(w, r)
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json") {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`<!doctype html><meta charset="utf-8"><title>Unauthorized</title><h1>Unauthorized</h1><p>Sign in at <a href="/admin/">/admin/</a> to view this.</p>`))
		return
	}
	httpx.WriteError(w, http.StatusUnauthorized, "unauthorized")
}
