package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testMux(t *testing.T, token string) http.Handler {
	t.Helper()
	preview := t.TempDir()
	upload := t.TempDir()
	if err := os.WriteFile(filepath.Join(preview, "index.html"), []byte("<html>draft-home</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(preview, "rates"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(preview, "rates", "index.html"), []byte("<html>draft-rates</html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(preview, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(preview, "assets", "theme.css"), []byte("body{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(upload, "shot.jpg"), []byte("JPEGDATA"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := New(token)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true}`)
	})
	mux.HandleFunc("/api/contact", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, `{"ok":true,"public":true}`)
	})
	mux.Handle("/api/admin/me", g.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.SetSessionCookie(w, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"ok":true,"role":"admin"}`)
	})))
	mux.HandleFunc("/api/admin/session", g.HandleSession)
	mux.Handle("/preview/", g.BearerOrCookie(http.StripPrefix("/preview/", FileServer(preview))))
	mux.Handle("/media/", g.BearerOrCookie(http.StripPrefix("/media/", FileServer(upload))))
	return mux
}

func TestUnauthenticatedPreviewMedia401(t *testing.T) {
	h := testMux(t, "test-admin-token")
	for _, path := range []string{"/preview/", "/preview/rates/", "/media/", "/media/shot.jpg", "/preview/assets/theme.css"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d want 401 body=%s", path, rec.Code, rec.Body.String())
		}
		if strings.Contains(strings.ToLower(rec.Body.String()), "shot.jpg") ||
			strings.Contains(rec.Body.String(), "JPEGDATA") ||
			strings.Contains(rec.Body.String(), "draft-home") {
			t.Errorf("%s: unauthenticated response leaked content: %s", path, rec.Body.String())
		}
	}
}

func TestWrongBearer401(t *testing.T) {
	h := testMux(t, "test-admin-token")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/preview/rates/", nil)
	req.Header.Set("Authorization", "Bearer wrong-token")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d want 401", rec.Code)
	}
}

func TestValidBearerPreviewAndMedia200(t *testing.T) {
	h := testMux(t, "test-admin-token")
	cases := []struct {
		path string
		want string
	}{
		{"/preview/rates/", "draft-rates"},
		{"/preview/", "draft-home"},
		{"/media/shot.jpg", "JPEGDATA"},
		{"/preview/assets/theme.css", "body{}"},
	}
	for _, tc := range cases {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		req.Header.Set("Authorization", "Bearer test-admin-token")
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: got %d want 200 body=%s", tc.path, rec.Code, rec.Body.String())
			continue
		}
		if !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("%s: body %q missing %q", tc.path, rec.Body.String(), tc.want)
		}
	}
}

func TestSessionCookieAllowsPreviewWithoutAuthorization(t *testing.T) {
	h := testMux(t, "test-admin-token")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/session", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("session: got %d body=%s", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	var session *http.Cookie
	for _, c := range cookies {
		if c.Name == CookieName {
			session = c
			break
		}
	}
	if session == nil {
		t.Fatal("missing session cookie")
	}
	if !session.HttpOnly {
		t.Error("cookie must be HttpOnly")
	}
	if session.Path != "/" {
		t.Errorf("cookie path %q", session.Path)
	}
	if session.SameSite != http.SameSiteLaxMode {
		t.Errorf("cookie SameSite %v want Lax", session.SameSite)
	}
	if session.Value == "test-admin-token" {
		t.Error("cookie must not contain the raw admin token")
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/preview/rates/", nil)
	req2.AddCookie(session)
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("cookie GET /preview/rates/: got %d body=%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), "draft-rates") {
		t.Fatalf("unexpected body %s", rec2.Body.String())
	}

	rec3 := httptest.NewRecorder()
	req3 := httptest.NewRequest(http.MethodGet, "/media/shot.jpg", nil)
	req3.AddCookie(session)
	h.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Fatalf("cookie GET /media/shot.jpg: got %d", rec3.Code)
	}
}

func TestMeSetsSessionCookie(t *testing.T) {
	h := testMux(t, "test-admin-token")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == CookieName && c.HttpOnly && c.Value != "" {
			found = true
		}
	}
	if !found {
		t.Fatal("GET /api/admin/me should Set-Cookie HttpOnly session")
	}
}

func TestMediaListingNotIndex(t *testing.T) {
	h := testMux(t, "test-admin-token")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/media/", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauth listing: got %d want 401", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "shot.jpg") {
		t.Fatalf("unauth listing leaked names: %s", rec.Body.String())
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/media/", nil)
	req2.Header.Set("Authorization", "Bearer test-admin-token")
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusForbidden && rec2.Code != http.StatusUnauthorized {
		t.Fatalf("auth listing: got %d want 403 (or 401), body=%s", rec2.Code, rec2.Body.String())
	}
	body := rec2.Body.String()
	if strings.Contains(body, "shot.jpg") || strings.Contains(body, "JPEGDATA") || strings.Contains(strings.ToLower(body), "<a href") {
		t.Fatalf("directory listing of originals: %s", body)
	}
}

func TestContactAndHealthStayPublic(t *testing.T) {
	h := testMux(t, "test-admin-token")

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("health: got %d", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodPost, "/api/contact", strings.NewReader(`{}`))
	h.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("contact: got %d want 200 (no token required)", rec2.Code)
	}
	if rec2.Code == http.StatusUnauthorized {
		t.Fatal("contact must not require admin token")
	}
}

func TestLogoutClearsCookie(t *testing.T) {
	h := testMux(t, "test-admin-token")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/admin/session", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("logout: got %d", rec.Code)
	}
	var cleared *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == CookieName {
			cleared = c
		}
	}
	if cleared == nil {
		t.Fatal("logout should Set-Cookie to clear")
	}
	if cleared.MaxAge >= 0 && cleared.Value != "" {
		t.Errorf("expected cleared cookie, maxAge=%d value=%q", cleared.MaxAge, cleared.Value)
	}
}

func TestSessionRejectsWrongBearer(t *testing.T) {
	h := testMux(t, "test-admin-token")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/session", nil)
	req.Header.Set("Authorization", "Bearer nope")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body["ok"] != false {
		t.Errorf("expected ok:false json, got %s", rec.Body.String())
	}
}

func TestHTMLAcceptGetsHTML401(t *testing.T) {
	h := testMux(t, "test-admin-token")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/preview/", nil)
	req.Header.Set("Accept", "text/html")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("content-type %q", ct)
	}
}

func TestSecureCookieWhenForwardedHTTPS(t *testing.T) {
	h := testMux(t, "test-admin-token")
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/session", nil)
	req.Header.Set("Authorization", "Bearer test-admin-token")
	req.Header.Set("X-Forwarded-Proto", "https")
	h.ServeHTTP(rec, req)
	var session *http.Cookie
	for _, c := range rec.Result().Cookies() {
		if c.Name == CookieName {
			session = c
		}
	}
	if session == nil {
		t.Fatal("missing cookie")
	}
	if !session.Secure {
		t.Error("Secure should be set when X-Forwarded-Proto=https")
	}
}
