package contact

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"sheyanova.art/api/internal/cms"
	"sheyanova.art/api/internal/httpx"
)

const (
	maxName    = 200
	maxEmail   = 254
	maxMessage = 8000
	maxBody    = 64 << 10
)

type Handler struct {
	Store           *cms.Store
	AllowedOrigins  []string
	Sender          Sender
	MinDwell        time.Duration
	TurnstileSecret string
	Limiter         *Limiter
	Now             func() time.Time
	VerifyTurnstile func(secret, token, ip string) error
}

func NewHandler(store *cms.Store, origins []string, sender Sender, turnstileSecret string) *Handler {
	return &Handler{
		Store:           store,
		AllowedOrigins:  origins,
		Sender:          sender,
		MinDwell:        2 * time.Second,
		TurnstileSecret: strings.TrimSpace(turnstileSecret),
		Limiter:         NewLimiter(10*time.Minute, 5),
		Now:             time.Now,
	}
}

type submitReq struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Message   string `json:"message"`
	Company   string `json:"company"`
	Website   string `json:"website"`
	Subject   string `json:"subject"`
	LoadedAt  int64  `json:"_t"`
	Turnstile string `json:"cf-turnstile-response"`
}

func (h *Handler) Submit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !h.originOK(r) {
		httpx.WriteError(w, http.StatusForbidden, "forbidden")
		return
	}
	ip := clientIP(r)
	if h.Limiter != nil && !h.Limiter.Allow(ip) {
		httpx.WriteError(w, http.StatusTooManyRequests, "Please wait a few minutes before sending another message.")
		return
	}
	req, err := decodeSubmit(r)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "Invalid request.")
		return
	}
	if field := honeypotField(req); field != "" {
		log.Printf("contact: honeypot ip=%s field=%s", ip, field)
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Thank you."})
		return
	}
	if err := h.checkDwell(req); err != nil {
		log.Printf("contact: dwell ip=%s: %v", ip, err)
		httpx.WriteError(w, http.StatusBadRequest, "Unable to send right now. Please try again.")
		return
	}
	name, emailAddr, message, verr := validateFields(req)
	if verr != "" {
		httpx.WriteError(w, http.StatusBadRequest, verr)
		return
	}
	if h.TurnstileSecret != "" {
		verify := h.VerifyTurnstile
		if verify == nil {
			verify = verifyTurnstile
		}
		if err := verify(h.TurnstileSecret, req.Turnstile, ip); err != nil {
			log.Printf("contact: turnstile ip=%s: %v", ip, err)
			httpx.WriteError(w, http.StatusBadRequest, "Unable to send right now. Please try again.")
			return
		}
	}

	to, err := h.Store.ContactRecipient()
	if err != nil {
		log.Printf("contact: recipient lookup: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "Could not send your message. Please try again later.")
		return
	}
	to = strings.TrimSpace(to)
	if to == "" || !validEmail(to) {
		log.Printf("contact: no contact_email configured")
		httpx.WriteError(w, http.StatusServiceUnavailable, "Contact form is not configured.")
		return
	}

	site, _ := h.Store.GetSettings()
	fromName := strings.TrimSpace(site.SiteName)
	if fromName == "" {
		fromName = "Sheyanova contact form"
	}
	fromAddr := ""
	if cs, ok := h.Sender.(ChainSender); ok {
		fromAddr = envelopeFrom(cs.SMTP, Message{})
	}

	msg := Message{
		To:        to,
		From:      fromAddr,
		FromName:  fromName,
		ReplyTo:   emailAddr,
		ReplyName: name,
		Subject:   "Contact form: " + name,
		Body:      formatBody(name, emailAddr, message, ip),
	}
	if h.Sender == nil {
		log.Printf("contact: sender nil")
		httpx.WriteError(w, http.StatusServiceUnavailable, "Could not send your message. Please try again later.")
		return
	}
	if err := h.Sender.Send(msg); err != nil {
		log.Printf("contact: send failed ip=%s to=%s: %v", ip, to, err)
		if errors.Is(err, ErrNotConfigured) {
			httpx.WriteError(w, http.StatusServiceUnavailable, "Could not send your message. Mail is not configured.")
			return
		}
		httpx.WriteError(w, http.StatusBadGateway, publicSendError(err))
		return
	}
	log.Printf("contact: sent ip=%s to=%s", ip, to)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "message": "Thank you. Your message has been sent."})
}

func decodeSubmit(r *http.Request) (submitReq, error) {
	ct := r.Header.Get("Content-Type")
	if strings.Contains(ct, "application/json") || ct == "" {
		raw, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
		if err != nil {
			return submitReq{}, err
		}
		if len(strings.TrimSpace(string(raw))) == 0 {
			return submitReq{}, errors.New("empty body")
		}
		var req submitReq
		if err := json.Unmarshal(raw, &req); err != nil {
			return submitReq{}, err
		}
		return req, nil
	}
	if err := r.ParseForm(); err != nil {
		return submitReq{}, err
	}
	t, _ := strconv.ParseInt(r.FormValue("_t"), 10, 64)
	return submitReq{
		Name:      r.FormValue("name"),
		Email:     r.FormValue("email"),
		Message:   r.FormValue("message"),
		Company:   r.FormValue("company"),
		Website:   r.FormValue("website"),
		Subject:   r.FormValue("subject"),
		LoadedAt:  t,
		Turnstile: r.FormValue("cf-turnstile-response"),
	}, nil
}

func isHoneypot(req submitReq) bool {
	return honeypotField(req) != ""
}

// honeypotField ignores "company": browsers and password managers autofill it
// even when the field is hidden, which fake-succeeded real contact submissions.
func honeypotField(req submitReq) string {
	if strings.TrimSpace(req.Website) != "" {
		return "website"
	}
	if strings.TrimSpace(req.Subject) != "" {
		return "subject"
	}
	return ""
}

func publicSendError(err error) string {
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "smtp auth"), strings.Contains(msg, "534"), strings.Contains(msg, "535"), strings.Contains(msg, "application-specific"):
		return "Could not send your message. Mail login failed. The site owner must set a Gmail app password."
	case strings.Contains(msg, "mail from"):
		return "Could not send your message. Mail server rejected the sender address."
	default:
		return "Could not send your message. Please try again later."
	}
}

func (h *Handler) checkDwell(req submitReq) error {
	min := h.MinDwell
	if min <= 0 {
		return nil
	}
	now := h.Now()
	if req.LoadedAt <= 0 {
		return errors.New("missing load timestamp")
	}
	loaded := time.UnixMilli(req.LoadedAt)
	if loaded.After(now.Add(5 * time.Minute)) {
		return errors.New("timestamp in the future")
	}
	if now.Sub(loaded) < min {
		return errors.New("submitted too quickly")
	}
	if now.Sub(loaded) > 48*time.Hour {
		return errors.New("timestamp too old")
	}
	return nil
}

func validateFields(req submitReq) (name, emailAddr, message, errMsg string) {
	name = strings.TrimSpace(req.Name)
	emailAddr = strings.TrimSpace(req.Email)
	message = strings.TrimSpace(req.Message)
	if name == "" {
		return "", "", "", "Please enter your name."
	}
	if utf8.RuneCountInString(name) > maxName {
		return "", "", "", "Name is too long."
	}
	if emailAddr == "" || !validEmail(emailAddr) {
		return "", "", "", "Please enter a valid email address."
	}
	if utf8.RuneCountInString(emailAddr) > maxEmail {
		return "", "", "", "Email is too long."
	}
	if message == "" {
		return "", "", "", "Please enter a message."
	}
	if utf8.RuneCountInString(message) > maxMessage {
		return "", "", "", "Message is too long."
	}
	if hasCRLF(name) || hasCRLF(emailAddr) {
		return "", "", "", "Invalid input."
	}
	return name, emailAddr, message, ""
}

func validEmail(s string) bool {
	a, err := mail.ParseAddress(s)
	if err != nil {
		return false
	}
	addr := a.Address
	at := strings.LastIndex(addr, "@")
	if at < 1 || at == len(addr)-1 {
		return false
	}
	host := addr[at+1:]
	return strings.Contains(host, ".")
}

func hasCRLF(s string) bool {
	return strings.ContainsAny(s, "\r\n")
}

func formatBody(name, email, message, ip string) string {
	var b strings.Builder
	b.WriteString("New message from the sheyanova.art contact form.\n\n")
	b.WriteString("Name: " + name + "\n")
	b.WriteString("Email: " + email + "\n")
	b.WriteString("IP: " + ip + "\n\n")
	b.WriteString(message)
	b.WriteString("\n")
	return b.String()
}

func (h *Handler) originOK(r *http.Request) bool {
	allowed := map[string]struct{}{}
	for _, o := range h.AllowedOrigins {
		o = strings.TrimRight(strings.TrimSpace(o), "/")
		if o == "" || o == "*" {
			continue
		}
		allowed[o] = struct{}{}
	}
	if len(allowed) == 0 {
		return true
	}
	origin := strings.TrimRight(strings.TrimSpace(r.Header.Get("Origin")), "/")
	if origin != "" {
		_, ok := allowed[origin]
		return ok
	}
	ref := strings.TrimSpace(r.Header.Get("Referer"))
	if ref == "" {
		return false
	}
	u, err := url.Parse(ref)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	_, ok := allowed[u.Scheme+"://"+u.Host]
	return ok
}

func clientIP(r *http.Request) string {
	if x := strings.TrimSpace(r.Header.Get("X-Real-IP")); x != "" {
		return strings.Split(x, ",")[0]
	}
	if x := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); x != "" {
		return strings.TrimSpace(strings.Split(x, ",")[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func verifyTurnstile(secret, token, ip string) error {
	if strings.TrimSpace(token) == "" {
		return errors.New("missing turnstile token")
	}
	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)
	if ip != "" {
		form.Set("remoteip", ip)
	}
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.PostForm("https://challenges.cloudflare.com/turnstile/v0/siteverify", form)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var out struct {
		Success    bool     `json:"success"`
		ErrorCodes []string `json:"error-codes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if !out.Success {
		if len(out.ErrorCodes) > 0 {
			return errors.New("turnstile failed: " + strings.Join(out.ErrorCodes, ","))
		}
		return errors.New("turnstile failed")
	}
	return nil
}

func TurnstileSecretFromEnv() string {
	return strings.TrimSpace(os.Getenv("TURNSTILE_SECRET_KEY"))
}
