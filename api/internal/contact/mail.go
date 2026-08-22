package contact

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var ErrNotConfigured = errors.New("mail is not configured: set SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASS, SMTP_FROM")

type SMTPConfig struct {
	Host string
	Port string
	User string
	Pass string
	From string
}

func SMTPFromEnv() SMTPConfig {
	return SMTPConfig{
		Host: strings.TrimSpace(os.Getenv("SMTP_HOST")),
		Port: strings.TrimSpace(os.Getenv("SMTP_PORT")),
		User: strings.TrimSpace(os.Getenv("SMTP_USER")),
		Pass: os.Getenv("SMTP_PASS"),
		From: strings.TrimSpace(os.Getenv("SMTP_FROM")),
	}
}

func (c SMTPConfig) Configured() bool {
	return strings.TrimSpace(c.Host) != ""
}

type Message struct {
	To        string
	From      string
	FromName  string
	ReplyTo   string
	ReplyName string
	Subject   string
	Body      string
	HTMLBody  string
}

type Sender interface {
	Send(m Message) error
}

// ChainSender uses SMTP when SMTP_HOST is set, otherwise sendmail(1).
type ChainSender struct {
	SMTP SMTPConfig
}

func (s ChainSender) Send(m Message) error {
	if s.SMTP.Configured() {
		return sendSMTP(s.SMTP, m)
	}
	if err := sendSendmail(m); err != nil {
		if errors.Is(err, errNoSendmail) {
			return ErrNotConfigured
		}
		return err
	}
	return nil
}

// envelopeFrom is the SMTP MAIL FROM / header From. Gmail (and most providers)
// reject a From address that is not the authenticated user unless a Send-as
// alias is verified, so SMTP_USER wins when it disagrees with SMTP_FROM.
func envelopeFrom(cfg SMTPConfig, m Message) string {
	user := strings.TrimSpace(cfg.User)
	from := strings.TrimSpace(cfg.From)
	if user != "" && from != "" && !strings.EqualFold(user, from) {
		return user
	}
	if from != "" {
		return from
	}
	if strings.TrimSpace(m.From) != "" {
		return strings.TrimSpace(m.From)
	}
	return user
}

func sendSMTP(cfg SMTPConfig, m Message) error {
	port := cfg.Port
	if port == "" {
		port = "587"
	}
	host := cfg.Host
	addr := net.JoinHostPort(host, port)
	tlsCfg := &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}

	dialer := net.Dialer{Timeout: 20 * time.Second}
	var (
		conn net.Conn
		err  error
	)
	if port == "465" {
		conn, err = tls.DialWithDialer(&dialer, "tcp", addr, tlsCfg)
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("smtp client: %w", err)
	}
	defer func() { _ = client.Close() }()

	if port != "465" {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsCfg); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}
	if cfg.User != "" {
		auth := smtp.PlainAuth("", cfg.User, cfg.Pass, host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	from := envelopeFrom(cfg, m)
	if from == "" {
		return fmt.Errorf("smtp: SMTP_FROM is empty")
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("smtp MAIL FROM: %w", err)
	}
	if err := client.Rcpt(m.To); err != nil {
		return fmt.Errorf("smtp RCPT TO: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp DATA: %w", err)
	}
	if _, err := w.Write([]byte(rfc822(cfg, m))); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}

var errNoSendmail = errors.New("sendmail not found")

func isBusyboxSendmail(bin string) bool {
	target, err := filepath.EvalSymlinks(bin)
	if err != nil {
		target = bin
	}
	return strings.Contains(strings.ToLower(target), "busybox")
}

func sendSendmail(m Message) error {
	bin, err := exec.LookPath("sendmail")
	if err != nil {
		for _, p := range []string{"/usr/sbin/sendmail", "/usr/bin/sendmail"} {
			if st, e := os.Stat(p); e == nil && !st.IsDir() {
				bin = p
				err = nil
				break
			}
		}
	}
	if err != nil || bin == "" || isBusyboxSendmail(bin) {
		return errNoSendmail
	}
	cmd := exec.Command(bin, "-i", "-t")
	cmd.Stdin = strings.NewReader(rfc822(SMTPConfig{}, m))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sendmail: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func rfc822(cfg SMTPConfig, m Message) string {
	fromAddr := envelopeFrom(cfg, m)
	fromName := strings.TrimSpace(m.FromName)
	if fromName == "" {
		fromName = "Sheyanova contact form"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", formatAddress(fromName, fromAddr))
	fmt.Fprintf(&b, "To: %s\r\n", m.To)
	if strings.TrimSpace(m.ReplyTo) != "" {
		fmt.Fprintf(&b, "Reply-To: %s\r\n", formatAddress(m.ReplyName, m.ReplyTo))
	}
	fmt.Fprintf(&b, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", m.Subject))
	fmt.Fprintf(&b, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	plain := crlfBody(m.Body)
	htmlPart := strings.TrimSpace(m.HTMLBody)
	if htmlPart == "" {
		fmt.Fprintf(&b, "Content-Type: text/plain; charset=utf-8\r\n")
		fmt.Fprintf(&b, "Content-Transfer-Encoding: 8bit\r\n")
		fmt.Fprintf(&b, "\r\n")
		b.WriteString(plain)
		return b.String()
	}
	var nonce [8]byte
	_, _ = rand.Read(nonce[:])
	boundary := "sheyanova-alt-" + hex.EncodeToString(nonce[:])
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%s\r\n", boundary)
	fmt.Fprintf(&b, "\r\n")
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: text/plain; charset=utf-8\r\n")
	fmt.Fprintf(&b, "Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(plain)
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	fmt.Fprintf(&b, "Content-Type: text/html; charset=utf-8\r\n")
	fmt.Fprintf(&b, "Content-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(crlfBody(htmlPart))
	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return b.String()
}

func crlfBody(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\n", "\r\n")
	if !strings.HasSuffix(s, "\r\n") {
		s += "\r\n"
	}
	return s
}

func formatAddress(name, email string) string {
	email = strings.TrimSpace(email)
	name = strings.TrimSpace(name)
	if name == "" {
		return email
	}
	return fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", name), email)
}
