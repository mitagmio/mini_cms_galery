package generate

import "testing"

func TestRenderContactFormPostsToPublicAPI(t *testing.T) {
	html := RenderContactForm(ContactFormInput{
		APIURL:         "https://api.sheyanova.art",
		FormID:         "contact_form_abc",
		NameLabel:      "Name",
		SuccessMessage: "Thanks!",
	})
	if !containsAll(html,
		`action="https://api.sheyanova.art/api/contact"`,
		`id="contact_form_abc"`,
		`name="website"`,
		`name="_t"`,
		`form-submit.js`,
		`SheyanovaForms.attach`,
		`Thanks!`,
	) {
		t.Fatalf("missing expected markup:\n%s", html)
	}
	if containsAll(html, `action="#"`) {
		t.Fatal("must not use relative # action")
	}
}

func TestRenderContactFormDefaultAPI(t *testing.T) {
	html := RenderContactForm(ContactFormInput{})
	if !containsAll(html, "https://api.sheyanova.art/api/contact") {
		t.Fatal(html)
	}
}

func TestRenderContactFormTurnstileOptional(t *testing.T) {
	plain := RenderContactForm(ContactFormInput{APIURL: "https://api.sheyanova.art"})
	if containsAll(plain, "challenges.cloudflare.com") {
		t.Fatal("turnstile should be absent without site key")
	}
	with := RenderContactForm(ContactFormInput{
		APIURL:           "https://api.sheyanova.art",
		TurnstileSiteKey: "0x-site",
	})
	if !containsAll(with, `data-sitekey="0x-site"`, "challenges.cloudflare.com") {
		t.Fatal(with)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !stringContains(s, p) {
			return false
		}
	}
	return true
}

func stringContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
