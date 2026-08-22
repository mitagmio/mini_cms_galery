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
	p["website"] = "http://spam.test"
	rec := postJSON(h.Submit, "https://sheyanova.art", p, nil)
	if rec.Code != 200 {
		t.Fatalf("code=%d", rec.Code)
	}
	if fs.n != 0 {
		t.Fatal("honeypot must not send")
	}
}

func TestSubmitCompanyAutofillStillSends(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	p := validPayload()
	p["company"] = "Acme Inc"
	rec := postJSON(h.Submit, "https://sheyanova.art", p, nil)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if fs.n != 1 {
		t.Fatal("autofilled company must not be treated as honeypot")
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

func TestSubmitAuthErrorSurfaced(t *testing.T) {
	fs := &fakeSender{err: errors.New("smtp auth: 534 5.7.9 Application-specific password required")}
	h, _ := testHandler(t, fs)
	rec := postJSON(h.Submit, "https://sheyanova.art", validPayload(), nil)
	if rec.Code != 502 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "app password") {
		t.Fatalf("body=%s", rec.Body.String())
	}
}

func TestEnvelopeFromUsesSMTPUserWhenAliasDiffers(t *testing.T) {
	got := envelopeFrom(SMTPConfig{User: "acct@gmail.com", From: "info@example.com"}, Message{})
	if got != "acct@gmail.com" {
		t.Fatalf("from=%s", got)
	}
	same := envelopeFrom(SMTPConfig{User: "info@example.com", From: "info@example.com"}, Message{})
	if same != "info@example.com" {
		t.Fatalf("same=%s", same)
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
	if !strings.Contains(raw, "Content-Type: text/plain") {
		t.Fatalf("plain-only mail should stay text/plain: %s", raw)
	}
	if strings.Contains(raw, "multipart/alternative") {
		t.Fatal("plain-only must not be multipart")
	}
}

func TestRFC822Multipart(t *testing.T) {
	raw := rfc822(SMTPConfig{From: "noreply@example.com"}, Message{
		To: "inbox@example.com", Subject: "Hi", Body: "Hello\nThere",
		HTMLBody: `<p>Hello</p><a href="https://drive.example/folder">https://drive.example/folder</a>`,
	})
	if !strings.Contains(raw, "multipart/alternative") {
		t.Fatalf("%s", raw)
	}
	if !strings.Contains(raw, "Content-Type: text/plain") || !strings.Contains(raw, "Content-Type: text/html") {
		t.Fatalf("missing parts: %s", raw)
	}
	if !strings.Contains(raw, "https://drive.example/folder") {
		t.Fatalf("url missing: %s", raw)
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

func ratesFashionPayload() map[string]any {
	p := validPayload()
	delete(p, "message")
	p["form"] = "rates_fashion"
	p["Contact"] = "Email"
	p["Imagelink"] = "https://drive.example/folder"
	p["Total"] = 3
	p["Final_delivery"] = "2026-09-01"
	p["Retouch_level"] = "2"
	p["Format"] = "JPG"
	p["Profile"] = "Adobe RGB (1998)"
	p["Brief"] = "Keep skin natural"
	p["colorwork"] = []string{"Basic RAW Development (Camera RAW or Capture One)"}
	return p
}

func TestSubmitRatesFashionSuccess(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	rec := postJSON(h.Submit, "https://sheyanova.art", ratesFashionPayload(), nil)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if fs.n != 1 {
		t.Fatalf("sent=%d", fs.n)
	}
	if fs.last.ReplyTo != "ada@example.com" {
		t.Fatalf("reply=%s", fs.last.ReplyTo)
	}
	if !strings.Contains(fs.last.Subject, "RATES / FASHION:") || !strings.Contains(fs.last.Subject, "Ada") {
		t.Fatalf("subject=%s", fs.last.Subject)
	}
	body := fs.last.Body
	if !strings.Contains(body, "New request from sheyanova.art RATES (FASHION)") {
		t.Fatalf("intro missing: %s", body)
	}
	if !strings.Contains(body, "Images:") || !strings.Contains(body, "https://drive.example/folder") {
		t.Fatalf("labeled image URL missing: %s", body)
	}
	if strings.Contains(body, "Imagelink:") {
		t.Fatalf("want question label Images, not raw key: %s", body)
	}
	if !strings.Contains(body, "Retouch level: 2 — Removing obvious blemishes") {
		t.Fatalf("retouch label missing: %s", body)
	}
	if !strings.Contains(body, "Color correction:") || !strings.Contains(body, "- Basic RAW Development") {
		t.Fatalf("checkbox group missing: %s", body)
	}
	if !strings.Contains(fs.last.HTMLBody, `<a href="https://drive.example/folder">`) {
		t.Fatalf("html link missing: %s", fs.last.HTMLBody)
	}
	if !strings.Contains(body, "Submitted from IP:") {
		t.Fatalf("missing IP in body")
	}
}

func TestSubmitContactStillRequiresMessage(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	p := validPayload()
	p["form"] = "contact"
	p["message"] = ""
	rec := postJSON(h.Submit, "https://sheyanova.art", p, nil)
	if rec.Code != 400 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if fs.n != 0 {
		t.Fatal("must not send")
	}
}

func TestSubmitRatesDoesNotRequireMessage(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	p := ratesFashionPayload()
	p["message"] = ""
	rec := postJSON(h.Submit, "https://sheyanova.art", p, nil)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestSubmitRatesUnknownForm(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	p := validPayload()
	p["form"] = "rates_cars"
	rec := postJSON(h.Submit, "https://sheyanova.art", p, nil)
	if rec.Code != 400 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if fs.n != 0 {
		t.Fatal("must not send")
	}
}

func TestSubmitRatesEmailAlias(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	p := ratesFashionPayload()
	delete(p, "email")
	p["Email"] = "alias@example.com"
	rec := postJSON(h.Submit, "https://sheyanova.art", p, nil)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if fs.last.ReplyTo != "alias@example.com" {
		t.Fatalf("reply=%s", fs.last.ReplyTo)
	}
}

func TestSubmitRatesCustomSchemaOption(t *testing.T) {
	fs := &fakeSender{}
	h, s := testHandler(t, fs)
	if err := s.EnsureSystemTemplates(); err != nil {
		t.Fatal(err)
	}
	cur, err := s.GetTemplate(cms.FormTemplateFashion)
	if err != nil {
		t.Fatal(err)
	}
	fields := cms.ParseFormFields(cur.DefaultBlocks)
	blocks := make([]map[string]any, 0, len(fields)+1)
	for _, f := range fields {
		if f.Name() == "colorwork" {
			opts := f.Options()
			raw := make([]any, 0, len(opts)+1)
			for _, o := range opts {
				raw = append(raw, map[string]any{"value": o.Value, "label": o.Label})
			}
			raw = append(raw, map[string]any{"value": "Studio LUT pass", "label": "Studio LUT pass"})
			f.Data["options"] = raw
		}
		blocks = append(blocks, map[string]any{"type": f.Type, "data": f.Data})
	}
	blocks = append(blocks, map[string]any{
		"type": cms.BlockFormText,
		"data": map[string]any{"name": "Studio_name", "label": "Studio", "required": false},
	})
	if _, err := s.PatchTemplate(cms.FormTemplateFashion, map[string]any{"default_blocks": blocks}); err != nil {
		t.Fatal(err)
	}
	p := ratesFashionPayload()
	p["colorwork"] = []string{"Studio LUT pass"}
	p["Studio_name"] = "Atelier North"
	rec := postJSON(h.Submit, "https://sheyanova.art", p, nil)
	if rec.Code != 200 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(fs.last.Body, "Studio LUT pass") {
		t.Fatalf("custom option missing: %s", fs.last.Body)
	}
	if !strings.Contains(fs.last.Body, "Studio") || !strings.Contains(fs.last.Body, "Atelier North") {
		t.Fatalf("custom field missing: %s", fs.last.Body)
	}
}

func TestSubmitRatesPhoneRequiredForWhatsApp(t *testing.T) {
	fs := &fakeSender{}
	h, _ := testHandler(t, fs)
	p := ratesFashionPayload()
	p["Contact"] = "WhatsApp"
	rec := postJSON(h.Submit, "https://sheyanova.art", p, nil)
	if rec.Code != 400 {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	p["Phone"] = "+15551212"
	rec = postJSON(h.Submit, "https://sheyanova.art", p, nil)
	if rec.Code != 200 {
		t.Fatalf("with phone code=%d body=%s", rec.Code, rec.Body.String())
	}
}
