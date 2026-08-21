package contact

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"sheyanova.art/api/internal/cms"
)

type fakeSender struct {
	last Message
	err  error
	n    int
}

func (f *fakeSender) Send(m Message) error {
	f.n++
	f.last = m
	return f.err
}

func testHandler(t *testing.T, sender Sender) (*Handler, *cms.Store) {
	t.Helper()
	s := testStore(t)
	_, err := s.PutSettings(cms.SiteSettings{
		SiteName:      "Test Site",
		ContactEmail:  "inbox@example.com",
		CanonicalBase: "https://sheyanova.art",
	})
	if err != nil {
		t.Fatal(err)
	}
	h := NewHandler(s, []string{"https://sheyanova.art", "https://www.sheyanova.art", "https://api.sheyanova.art"}, sender, "")
	h.MinDwell = 2 * time.Second
	h.Now = func() time.Time { return time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC) }
	return h, s
}

func testStore(t *testing.T) *cms.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := cms.Open(dir, dir+"/up")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func postJSON(h http.HandlerFunc, origin string, body any, extra map[string]string) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/contact", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	if origin != "" {
		req.Header.Set("Origin", origin)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func validPayload() map[string]any {
	return map[string]any{
		"name":    "Ada Lovelace",
		"email":   "ada@example.com",
		"message": "Hello from the form",
		"_t":      time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC).Add(-5 * time.Second).UnixMilli(),
	}
}

func TestSubmitSuccess(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	rec := postJSON(h.Submit, "https://sheyanova.art", validPayload(), nil)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if fs.n != 1 {
		t.Fatalf("sent=%d", fs.n)
	}
	if fs.last.To != "inbox@example.com" || fs.last.ReplyTo != "ada@example.com" {
		t.Fatalf("mail=%+v", fs.last)
	}
	if !strings.Contains(fs.last.Body, "Hello from the form") {
		t.Fatalf("body=%s", fs.last.Body)
	}
}

func TestSubmitValidation(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	cases := []struct {
		mut  func(map[string]any)
		want int
	}{
		{func(m map[string]any) { m["name"] = "" }, 400},
		{func(m map[string]any) { m["email"] = "not-an-email" }, 400},
		{func(m map[string]any) { m["message"] = "" }, 400},
		{func(m map[string]any) { m["email"] = "a\r\nb@x.com" }, 400},
	}
	for i, tc := range cases {
		p := validPayload()
		tc.mut(p)
		rec := postJSON(h.Submit, "https://sheyanova.art", p, nil)
		if rec.Code != tc.want {
			t.Fatalf("case %d code=%d body=%s", i, rec.Code, rec.Body.String())
		}
		if fs.n != 0 {
			t.Fatalf("case %d sent mail", i)
		}
	}
}

func TestSubmitHoneypotPretendsOK(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	p := validPayload()
	p["company"] = "http://spam.test"
	rec := postJSON(h.Submit, "https://sheyanova.art", p, nil)
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
	if fs.n != 0 {
		t.Fatal("honeypot must not send")
	}
}

func TestSubmitTooFast(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	p := validPayload()
	p["_t"] = h.Now().UnixMilli()
	rec := postJSON(h.Submit, "https://sheyanova.art", p, nil)
	if rec.Code != 400 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if fs.n != 0 {
		t.Fatal("too-fast must not send")
	}
}

func TestSubmitBadOrigin(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	rec := postJSON(h.Submit, "https://evil.example", validPayload(), nil)
	if rec.Code != 403 {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestSubmitRefererFallback(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	rec := postJSON(h.Submit, "", validPayload(), map[string]string{
		"Referer": "https://sheyanova.art/contact/",
	})
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSubmitRateLimit(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	h.Limiter = NewLimiter(time.Hour, 2)
	if rec := postJSON(h.Submit, "https://sheyanova.art", validPayload(), nil); rec.Code != 200 {
		t.Fatalf("1: %d %s", rec.Code, rec.Body.String())
	}
	if rec := postJSON(h.Submit, "https://sheyanova.art", validPayload(), nil); rec.Code != 200 {
		t.Fatalf("2: %d", rec.Code)
	}
	if rec := postJSON(h.Submit, "https://sheyanova.art", validPayload(), nil); rec.Code != 429 {
		t.Fatalf("3: %d", rec.Code)
	}
}

func TestSubmitNoRecipient(t *testing.T) {
	fs := &fakeSender{}
	s := testStore(t)
	h := NewHandler(s, []string{"https://sheyanova.art"}, fs, "")
	h.MinDwell = 0
	rec := postJSON(h.Submit, "https://sheyanova.art", validPayload(), nil)
	if rec.Code != 503 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSubmitMailNotConfigured(t *testing.T) {
	fs := &fakeSender{err: ErrNotConfigured}
	h, _ := testHandler(t, fs)
	rec := postJSON(h.Submit, "https://sheyanova.art", validPayload(), nil)
	if rec.Code != 503 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSubmitSendError(t *testing.T) {
	fs := &fakeSender{err: errors.New("smtp 550")}
	h, _ := testHandler(t, fs)
	rec := postJSON(h.Submit, "https://sheyanova.art", validPayload(), nil)
	if rec.Code != 502 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSubmitMethod(t *testing.T) {
	h, _ := testHandler(t, &fakeSender{})
	req := httptest.NewRequest(http.MethodGet, "/api/contact", nil)
	rec := httptest.NewRecorder()
	h.Submit(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestSubmitTurnstileRequired(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	h.TurnstileSecret = "secret"
	h.VerifyTurnstile = func(secret, token, ip string) error {
		if token != "ok-token" {
			return errors.New("bad token")
		}
		return nil
	}
	p := validPayload()
	rec := postJSON(h.Submit, "https://sheyanova.art", p, nil)
	if rec.Code != 400 {
		t.Fatalf("missing token code=%d", rec.Code)
	}
	p["cf-turnstile-response"] = "ok-token"
	rec = postJSON(h.Submit, "https://sheyanova.art", p, nil)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSubmitBlockMailtoFallback(t *testing.T) {
	fs := &fakeSender{}
	s := testStore(t)
	if _, err := s.PutSettings(cms.SiteSettings{SiteName: "Test Site"}); err != nil {
		t.Fatal(err)
	}
	p, err := s.CreatePage(cms.Page{Slug: "contact", Title: "CONTACT", Theme: cms.ThemeTextContent, Status: "published"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReplaceBlocks(p.ID, []cms.Block{{
		Type: cms.BlockContactForm,
		Data: cms.MustJSON(map[string]any{"mailto": "from-block@example.com"}),
	}}); err != nil {
		t.Fatal(err)
	}
	if err := s.BackfillContactEmail(); err != nil {
		t.Fatal(err)
	}
	st, err := s.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	if st.ContactEmail != "from-block@example.com" {
		t.Fatalf("backfill=%q", st.ContactEmail)
	}
	h := NewHandler(s, []string{"https://sheyanova.art"}, fs, "")
	h.MinDwell = 0
	rec := postJSON(h.Submit, "https://sheyanova.art", validPayload(), nil)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if fs.last.To != "from-block@example.com" {
		t.Fatalf("to=%s", fs.last.To)
	}
}

func TestRFC822Sanity(t *testing.T) {
	raw := rfc822(SMTPConfig{From: "noreply@example.com"}, Message{
		To: "inbox@example.com", FromName: "Site", ReplyTo: "ada@example.com", ReplyName: "Ada",
		Subject: "Hi", Body: "Hello\nThere",
	})
	if !strings.Contains(raw, "To: inbox@example.com") || !strings.Contains(raw, "Reply-To:") {
		t.Fatalf("%s", raw)
	}
}

func TestLimiter(t *testing.T) {
	l := NewLimiter(time.Hour, 1)
	if !l.Allow("a") || l.Allow("a") {
		t.Fatal("limit")
	}
	if !l.Allow("b") {
		t.Fatal("other key")
	}
}

func TestDecodeForm(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/contact", strings.NewReader("name=Ada&email=ada@example.com&message=Hi&_t=1"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	got, err := decodeSubmit(req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Ada" || got.LoadedAt != 1 {
		t.Fatalf("%+v", got)
	}
	_ = io.Discard
}
